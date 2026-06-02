package router

import (
	"context"
	"errors"
	"fmt"

	"github.com/user1024/auto-router/internal/cache"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/db"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"go.uber.org/zap"
)

type RoutingEngine struct {
	cfg        *config.Config
	classifier *Classifier
}

func NewRoutingEngine(cfg *config.Config) *RoutingEngine {
	return &RoutingEngine{
		cfg:        cfg,
		classifier: NewClassifier(cfg.Providers.Ollama),
	}
}

func (e *RoutingEngine) Route(ctx context.Context, req provider.ChatRequest, apiKey string) (string, string, db.DBAuditLog, error) {
	var targetProvider string
	var targetModel string
	var auditLog db.DBAuditLog
	auditLog.OriginalModel = req.Model

	if req.Model == "auto" {
		auditLog.RoutingType = "auto"

		classification := e.classifier.Classify(ctx, req.Messages)
		auditLog.Complexity = classification.Complexity
		auditLog.Confidence = classification.Confidence
		auditLog.Reason = classification.Reason

		var idealProvider string
		switch classification.Complexity {
		case "low":
			idealProvider = "ollama"
			targetModel = e.getFirstEnabledModel("ollama")
		case "medium":
			if _, err := provider.GetProvider("agy"); err == nil {
				idealProvider = "agy"
				targetModel = e.getFirstEnabledModel("agy")
			} else {
				idealProvider = "gemini"
				targetModel = e.getFirstEnabledModel("gemini")
			}
		case "high":
			if _, err := provider.GetProvider("subscription"); err == nil {
				idealProvider = "subscription"
				targetModel = e.getFirstEnabledModel("subscription")
			} else {
				idealProvider = "openai"
				targetModel = e.getFirstEnabledModel("openai")
			}
		default:
			idealProvider = "ollama"
			targetModel = e.getFirstEnabledModel("ollama")
		}

		score := GetProviderPerformanceScore(ctx, idealProvider)
		if score < 0.5 {
			bestAlternative := idealProvider
			bestScore := score
			alternatives := []string{"ollama", "gemini", "openai"}
			for _, alt := range alternatives {
				if alt == idealProvider {
					continue
				}
				if _, err := provider.GetProvider(alt); err != nil {
					continue
				}
				altScore := GetProviderPerformanceScore(ctx, alt)
				if altScore > bestScore && altScore >= 0.5 {
					bestScore = altScore
					bestAlternative = alt
				}
			}
			if bestAlternative != idealProvider {
				logger.Log.Info("Rerouted based on provider performance score profile", zap.String("original", idealProvider), zap.String("selected", bestAlternative), zap.Float64("score", bestScore))
				idealProvider = bestAlternative
				targetModel = e.getFirstEnabledModel(bestAlternative)
				auditLog.Reason = fmt.Sprintf("%s (Dynamic profile rerouted to %s)", auditLog.Reason, bestAlternative)
			}
		}

		targetProvider = e.enforceBudgetAndDowngrade(ctx, apiKey, idealProvider)
		if targetProvider != idealProvider {
			auditLog.Reason = fmt.Sprintf("%s (Auto-downgraded from %s due to budget limits)", auditLog.Reason, idealProvider)
			targetModel = e.getFirstEnabledModel(targetProvider)
		}

		if isHealthy, _, _ := cache.GetProviderHealth(ctx, targetProvider); !isHealthy {
			fallbackProvider := e.findHealthyFallback(ctx, targetProvider)
			logger.Log.Warn("Target provider unhealthy, falling back", zap.String("target", targetProvider), zap.String("fallback", fallbackProvider))
			auditLog.Reason = fmt.Sprintf("%s (Felled back from unhealthy provider %s)", auditLog.Reason, targetProvider)
			targetProvider = fallbackProvider
			targetModel = e.getFirstEnabledModel(targetProvider)
		}

	} else {
		auditLog.RoutingType = "explicit"
		auditLog.Reason = "Explicit model requested"

		regModel, found := GetRegistryModel(req.Model)
		if !found {
			return "", "", auditLog, fmt.Errorf("model not found in registry: %s", req.Model)
		}

		targetProvider = regModel.Provider
		targetModel = regModel.Name

		downgradedProvider := e.enforceBudgetAndDowngrade(ctx, apiKey, targetProvider)
		if downgradedProvider != targetProvider {
			auditLog.Reason = fmt.Sprintf("Auto-downgraded from %s due to budget limits", targetProvider)
			targetProvider = downgradedProvider
			targetModel = e.getFirstEnabledModel(targetProvider)
		}

		if isHealthy, _, _ := cache.GetProviderHealth(ctx, targetProvider); !isHealthy {
			fallbackProvider := e.findHealthyFallback(ctx, targetProvider)
			logger.Log.Warn("Explicit provider unhealthy, falling back", zap.String("target", targetProvider), zap.String("fallback", fallbackProvider))
			auditLog.Reason = fmt.Sprintf("%s (Felled back from unhealthy provider %s)", auditLog.Reason, targetProvider)
			targetProvider = fallbackProvider
			targetModel = e.getFirstEnabledModel(targetProvider)
		}
	}

	auditLog.ChosenProvider = targetProvider
	auditLog.ChosenModel = targetModel

	if targetProvider == "" || targetModel == "" {
		return "", "", auditLog, errors.New("failed to find suitable provider or model after routing and fallback evaluations")
	}

	return targetProvider, targetModel, auditLog, nil
}

func (e *RoutingEngine) getFirstEnabledModel(prov string) string {
	switch prov {
	case "openai":
		if len(e.cfg.Providers.OpenAI.Models) > 0 {
			return e.cfg.Providers.OpenAI.Models[0].Name
		}
	case "gemini":
		if len(e.cfg.Providers.Gemini.Models) > 0 {
			return e.cfg.Providers.Gemini.Models[0].Name
		}
	case "ollama":
		if len(e.cfg.Providers.Ollama.Models) > 0 {
			return e.cfg.Providers.Ollama.Models[0].Name
		}
	case "subscription":
		if len(e.cfg.Providers.Subscription.Models) > 0 {
			return e.cfg.Providers.Subscription.Models[0].Name
		}
	case "agy":
		if len(e.cfg.Providers.Agy.Models) > 0 {
			return e.cfg.Providers.Agy.Models[0].Name
		}
	}
	return e.cfg.Routing.DefaultModel
}

func (e *RoutingEngine) enforceBudgetAndDowngrade(ctx context.Context, apiKey string, targetProv string) string {
	if apiKey == "" {
		return targetProv
	}

	dCost, mCost, err := cache.GetCachedBudgetUsage(ctx, apiKey)
	if err != nil {
		budget, dbErr := db.GetBudget(ctx, apiKey)
		if dbErr != nil {
			return targetProv
		}
		dCost = budget.DailyUsage
		mCost = budget.MonthlyUsage
	}

	budget, dbErr := db.GetBudget(ctx, apiKey)
	if dbErr != nil {
		return targetProv
	}

	if targetProv == "openai" {
		if dCost >= budget.DailyLimit || mCost >= budget.MonthlyLimit {
			logger.Log.Info("Budget exceeded for OpenAI, downgrading to Gemini", zap.String("api_key", apiKey))
			targetProv = "gemini"
		}
	}

	if targetProv == "gemini" {
		if dCost >= budget.DailyLimit || mCost >= budget.MonthlyLimit {
			logger.Log.Info("Budget exceeded for Gemini, downgrading to Ollama", zap.String("api_key", apiKey))
			targetProv = "ollama"
		}
	}

	return targetProv
}

func (e *RoutingEngine) findHealthyFallback(ctx context.Context, unhealthyProv string) string {
	chain, exists := e.cfg.Routing.Failover.Chains[unhealthyProv]
	if !exists {
		chain = e.cfg.Routing.Failover.Chains["default"]
	}

	for _, prov := range chain {
		if isHealthy, _, _ := cache.GetProviderHealth(ctx, prov); isHealthy {
			_, err := provider.GetProvider(prov)
			if err == nil {
				return prov
			}
		}
	}

	return "ollama"
}
