package router

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"github.com/user1024/auto-router/internal/cache"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/db"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"go.uber.org/zap"
)

var (
	breakers   = make(map[string]*gobreaker.CircuitBreaker)
	breakersMu sync.RWMutex
)

func InitCircuitBreakers(cfg *config.Config) {
	breakersMu.Lock()
	defer breakersMu.Unlock()

	maxFailures := uint32(cfg.Routing.Failover.CircuitBreaker.MaxFailures)
	timeout := time.Duration(cfg.Routing.Failover.CircuitBreaker.TimeoutSeconds) * time.Second

	providersList := []string{"openai", "gemini", "ollama", "subscription", "agy"}
	for _, name := range providersList {
		st := gobreaker.Settings{
			Name:        name + "-breaker",
			MaxRequests: 3,
			Interval:    60 * time.Second,
			Timeout:     timeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= maxFailures
			},
			OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
				logger.Log.Warn("Circuit breaker state change", zap.String("name", name), zap.String("from", from.String()), zap.String("to", to.String()))
			},
		}
		breakers[name] = gobreaker.NewCircuitBreaker(st)
	}
}

func GetCircuitBreaker(name string) (*gobreaker.CircuitBreaker, bool) {
	breakersMu.RLock()
	defer breakersMu.RUnlock()
	cb, ok := breakers[name]
	return cb, ok
}

type FailoverManager struct {
	cfg    *config.Config
	engine *RoutingEngine
}

func NewFailoverManager(cfg *config.Config, engine *RoutingEngine) *FailoverManager {
	return &FailoverManager{
		cfg:    cfg,
		engine: engine,
	}
}

func (f *FailoverManager) ExecuteChat(ctx context.Context, req provider.ChatRequest, apiKey string) (*provider.ChatResponse, string, string, db.DBAuditLog, error) {
	targetProvider, targetModel, auditLog, err := f.engine.Route(ctx, req, apiKey)
	if err != nil {
		return nil, "", "", db.DBAuditLog{}, fmt.Errorf("routing failure: %w", err)
	}

	chain, exists := f.cfg.Routing.Failover.Chains[targetProvider]
	if !exists {
		chain = f.cfg.Routing.Failover.Chains["default"]
	}

	providersToTry := []string{targetProvider}
	for _, p := range chain {
		if p != targetProvider {
			providersToTry = append(providersToTry, p)
		}
	}

	var lastErr error
	for _, provName := range providersToTry {
		prov, err := provider.GetProvider(provName)
		if err != nil {
			continue
		}

		cb, ok := GetCircuitBreaker(provName)
		if !ok {
			continue
		}

		// Use the routed model for the primary provider; fall back to the
		// provider's first configured model for any fallback in the chain.
		provModel := targetModel
		if provName != targetProvider {
			provModel = f.engine.getFirstEnabledModel(provName)
		}
		provReq := req
		provReq.Model = provModel

		response, execErr := cb.Execute(func() (interface{}, error) {
			var chatResp *provider.ChatResponse
			var runErr error

			maxRetries := f.cfg.Routing.Failover.MaxRetries
			backoff := time.Duration(f.cfg.Routing.Failover.RetryBackoffMs) * time.Millisecond

			for attempt := 0; attempt <= maxRetries; attempt++ {
				start := time.Now()
				chatResp, runErr = prov.Chat(ctx, provReq)
				latencyMs := time.Since(start).Milliseconds()

				if runErr == nil {
					_ = cache.SetProviderStatus(ctx, provName, true, latencyMs)
					return chatResp, nil
				}

				logger.Log.Warn("Request attempt failed", zap.String("provider", provName), zap.Int("attempt", attempt), zap.Error(runErr))
				_ = cache.SetProviderStatus(ctx, provName, false, latencyMs)

				if attempt < maxRetries {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(backoff):
						backoff *= 2
					}
				}
			}
			return nil, runErr
		})

		lastErr = execErr
		if lastErr == nil {
			if chatResp, ok := response.(*provider.ChatResponse); ok {
				return chatResp, provName, provModel, auditLog, nil
			}
			return nil, provName, provModel, auditLog, errors.New("invalid cast type for provider chat response")
		}

		logger.Log.Error("Circuit breaker failed for provider, switching to fallback", zap.String("provider", provName), zap.Error(lastErr))
	}

	return nil, "", "", auditLog, fmt.Errorf("all providers in failover chain exhausted. Last error: %w", lastErr)
}

func (f *FailoverManager) ExecuteStream(ctx context.Context, req provider.ChatRequest, apiKey string, sseChan chan<- string) (string, string, db.DBAuditLog, error) {
	targetProvider, targetModel, auditLog, err := f.engine.Route(ctx, req, apiKey)
	if err != nil {
		return "", "", db.DBAuditLog{}, fmt.Errorf("routing failure: %w", err)
	}

	chain, exists := f.cfg.Routing.Failover.Chains[targetProvider]
	if !exists {
		chain = f.cfg.Routing.Failover.Chains["default"]
	}

	providersToTry := []string{targetProvider}
	for _, p := range chain {
		if p != targetProvider {
			providersToTry = append(providersToTry, p)
		}
	}

	var lastErr error
	for _, provName := range providersToTry {
		prov, err := provider.GetProvider(provName)
		if err != nil {
			continue
		}

		cb, ok := GetCircuitBreaker(provName)
		if !ok {
			continue
		}

		provModel := targetModel
		if provName != targetProvider {
			provModel = f.engine.getFirstEnabledModel(provName)
		}
		provReq := req
		provReq.Model = provModel

		_, execErr := cb.Execute(func() (interface{}, error) {
			var runErr error
			maxRetries := f.cfg.Routing.Failover.MaxRetries
			backoff := time.Duration(f.cfg.Routing.Failover.RetryBackoffMs) * time.Millisecond

			for attempt := 0; attempt <= maxRetries; attempt++ {
				start := time.Now()
				runErr = prov.Stream(ctx, provReq, sseChan)
				latencyMs := time.Since(start).Milliseconds()

				if runErr == nil {
					_ = cache.SetProviderStatus(ctx, provName, true, latencyMs)
					return nil, nil
				}

				logger.Log.Warn("Stream request attempt failed", zap.String("provider", provName), zap.Int("attempt", attempt), zap.Error(runErr))
				_ = cache.SetProviderStatus(ctx, provName, false, latencyMs)

				if attempt < maxRetries {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(backoff):
						backoff *= 2
					}
				}
			}
			return nil, runErr
		})

		lastErr = execErr
		if lastErr == nil {
			return provName, provModel, auditLog, nil
		}

		logger.Log.Error("Circuit breaker failed for provider stream, switching to fallback", zap.String("provider", provName), zap.Error(lastErr))
	}

	return "", "", auditLog, fmt.Errorf("all providers in stream failover chain exhausted. Last error: %w", lastErr)
}
