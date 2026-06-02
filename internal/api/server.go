package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user1024/auto-router/internal/agent"
	"github.com/user1024/auto-router/internal/cache"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/db"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"github.com/user1024/auto-router/internal/router"
	"github.com/user1024/auto-router/internal/telemetry"
	"go.uber.org/zap"
)

type APIServer struct {
	cfg          *config.Config
	failover     *router.FailoverManager
	engine       *router.RoutingEngine
	orchestrator *agent.Orchestrator
}

func NewAPIServer(cfg *config.Config, fm *router.FailoverManager, re *router.RoutingEngine, orch *agent.Orchestrator) *APIServer {
	return &APIServer{
		cfg:          cfg,
		failover:     fm,
		engine:       re,
		orchestrator: orch,
	}
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("req_%s", hex.EncodeToString(b))
}

func (s *APIServer) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.corsMiddleware())

	r.GET("/health", s.handleHealth)
	r.GET("/metrics", s.handleMetrics)

	v1 := r.Group("/v1")
	v1.Use(s.authMiddleware())
	v1.Use(s.rateLimitMiddleware())
	{
		v1.POST("/chat/completions", s.handleChatCompletions)
		v1.POST("/embeddings", s.handleEmbeddings)
		v1.GET("/models", s.handleModels)
		v1.GET("/providers", s.handleProviders)
		v1.GET("/usage/logs", s.handleUsageLogs)
		v1.GET("/logs", s.handleRequestLogs)
		v1.GET("/orchestration/:requestID", s.handleOrchestrationStatus)

		admin := v1.Group("/admin")
		admin.Use(s.adminOnlyMiddleware())
		{
			admin.GET("/providers", s.handleAdminGetProviders)
			admin.PUT("/providers/:name", s.handleAdminUpdateProvider)
			admin.POST("/providers/:name/test", s.handleAdminTestProvider)
		}
	}

	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	logger.Log.Info("API Gateway starting HTTP server", zap.String("addr", addr))
	return r.Run(addr)
}

func (s *APIServer) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *APIServer) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be Bearer token"})
			c.Abort()
			return
		}

		tokenStr := parts[1]
		var matchedKey *config.APIKeyConfig
		for _, keyCfg := range s.cfg.Server.APIKeys {
			if keyCfg.Key == tokenStr {
				matchedKey = &keyCfg
				break
			}
		}

		if matchedKey == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Set("api_key", matchedKey.Key)
		c.Set("role", matchedKey.Role)
		c.Set("rate_limit", matchedKey.RateLimit)
		c.Next()
	}
}

func (s *APIServer) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetString("api_key")
		limit := c.GetInt("rate_limit")

		allowed, remaining, err := cache.RateLimitCheck(c.Request.Context(), apiKey, limit)
		if err != nil {
			logger.Log.Error("Rate limit check failed", zap.Error(err))
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": "Rate limit exceeded. Please retry later.",
					"type":    "rate_limit_error",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (s *APIServer) handleChatCompletions(c *gin.Context) {
	reqID := generateRequestID()
	apiKey := c.GetString("api_key")

	var req provider.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Messages cannot be empty"})
		return
	}

	ctx := c.Request.Context()
	logger.Log.Info("Received chat completions request", zap.String("id", reqID), zap.String("model", req.Model), zap.Bool("stream", req.Stream))

	if req.Model == "orchestrated" {
		start := time.Now()
		resp, err := s.orchestrator.ExecuteOrchestrated(ctx, req, reqID)
		duration := time.Since(start)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		dbReq := db.DBRequest{
			ID:               reqID,
			Provider:         "openai",
			Model:            "orchestrated",
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			Cost:             0.0,
			LatencyMs:        int(duration.Milliseconds()),
			Status:           http.StatusOK,
		}
		_ = db.SaveRequest(ctx, dbReq)

		c.JSON(http.StatusOK, resp)
		return
	}

	if !req.Stream {
		promptContent := req.Messages[len(req.Messages)-1].Content
		cacheKey := cache.GenerateCacheKey(promptContent, req.Model, false)
		cachedBytes, err := cache.GetResponseCache(ctx, cacheKey)
		if err == nil && cachedBytes != nil {
			var cachedResp provider.ChatResponse
			if err := json.Unmarshal(cachedBytes, &cachedResp); err == nil {
				cachedResp.ID = reqID
				c.Header("X-Cache", "HIT")
				telemetry.CacheHitsTotal.WithLabelValues(req.Model, "hit").Inc()
				c.JSON(http.StatusOK, cachedResp)
				return
			}
		}
		telemetry.CacheHitsTotal.WithLabelValues(req.Model, "miss").Inc()
		c.Header("X-Cache", "MISS")
	}

	if req.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")

		sseChan := make(chan string, 100)
		errChan := make(chan error, 1)

		go func() {
			defer close(sseChan)
			_, _, _, err := s.failover.ExecuteStream(ctx, req, apiKey, sseChan)
			if err != nil {
				errChan <- err
			}
		}()

		c.Stream(func(w io.Writer) bool {
			select {
			case line, ok := <-sseChan:
				if !ok {
					return false
				}
				c.Writer.Write([]byte(line + "\n\n"))
				c.Writer.Flush()
				return true
			case err := <-errChan:
				c.Writer.Write([]byte(fmt.Sprintf("data: {\"error\": %q}\n\n", err.Error())))
				c.Writer.Flush()
				return false
			case <-ctx.Done():
				return false
			}
		})
		return
	}

	start := time.Now()
	resp, chosenProv, chosenModel, routingAudit, err := s.failover.ExecuteChat(ctx, req, apiKey)
	duration := time.Since(start)

	routingType := "explicit"
	if req.Model == "auto" {
		routingType = "auto"
	}

	if err != nil {
		dbErr := db.SaveRequest(ctx, db.DBRequest{
			ID:           reqID,
			Provider:     "error",
			Model:        req.Model,
			Status:       http.StatusInternalServerError,
			ErrorMessage: err.Error(),
			LatencyMs:    int(duration.Milliseconds()),
		})
		if dbErr != nil {
			logger.Log.Error("Failed to save error request to database", zap.Error(dbErr))
		}

		telemetry.RequestsTotal.WithLabelValues("error", req.Model, "500", routingType).Inc()
		telemetry.RequestDuration.WithLabelValues("error", req.Model, "500", routingType).Observe(duration.Seconds())

		if chosenProv != "" {
			router.LogCallMetrics(ctx, chosenProv, duration.Milliseconds(), true, "", false)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	regModel, found := router.GetRegistryModel(chosenModel)
	var totalCost float64 = 0.0
	if found {
		promptCost := (float64(resp.Usage.PromptTokens) / 1000.0) * regModel.CostPer1kPrompt
		completionCost := (float64(resp.Usage.CompletionTokens) / 1000.0) * regModel.CostPer1kCompletion
		totalCost = promptCost + completionCost
		resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	}

	dbReq := db.DBRequest{
		ID:               reqID,
		Provider:         chosenProv,
		Model:            chosenModel,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		Cost:             totalCost,
		LatencyMs:        int(duration.Milliseconds()),
		Status:           http.StatusOK,
	}
	_ = db.SaveRequest(ctx, dbReq)

	_ = db.SaveAuditLog(ctx, db.DBAuditLog{
		RequestID:      reqID,
		OriginalModel:  req.Model,
		ChosenProvider: chosenProv,
		ChosenModel:    chosenModel,
		RoutingType:    routingAudit.RoutingType,
		Complexity:     routingAudit.Complexity,
		Confidence:     routingAudit.Confidence,
		Reason:         routingAudit.Reason,
	})

	if apiKey != "" {
		_, _, _ = cache.TrackBudgetUsage(ctx, apiKey, totalCost)
		_ = db.UpdateBudgetUsage(ctx, apiKey, totalCost)
	}

	// Feed dynamic performance profiling loop (latency and syntax validity checks)
	isJSONRequested := false
	if len(req.Messages) > 0 {
		originalPrompt := req.Messages[len(req.Messages)-1].Content
		isJSONRequested = strings.Contains(strings.ToLower(originalPrompt), "json")
	}
	if len(resp.Choices) > 0 {
		responseText := resp.Choices[0].Message.Content
		router.LogCallMetrics(ctx, chosenProv, duration.Milliseconds(), false, responseText, isJSONRequested)
	}

	telemetry.RequestsTotal.WithLabelValues(chosenProv, chosenModel, "200", routingType).Inc()
	telemetry.RequestDuration.WithLabelValues(chosenProv, chosenModel, "200", routingType).Observe(duration.Seconds())
	telemetry.TokensTotal.WithLabelValues(chosenProv, chosenModel, "prompt").Add(float64(resp.Usage.PromptTokens))
	telemetry.TokensTotal.WithLabelValues(chosenProv, chosenModel, "completion").Add(float64(resp.Usage.CompletionTokens))
	telemetry.CostTotal.WithLabelValues(chosenProv, chosenModel, apiKey).Add(totalCost)

	promptContent := req.Messages[len(req.Messages)-1].Content
	cacheKey := cache.GenerateCacheKey(promptContent, req.Model, false)
	respBytes, err := json.Marshal(resp)
	if err == nil {
		_ = cache.SetResponseCache(ctx, cacheKey, respBytes)
	}

	c.JSON(http.StatusOK, resp)
}

func (s *APIServer) handleEmbeddings(c *gin.Context) {
	apiKey := c.GetString("api_key")
	var reqBody map[string]interface{}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := provider.GetProvider("openai")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OpenAI provider is disabled"})
		return
	}

	logger.Log.Info("Embedding request received", zap.String("api_key", apiKey))
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Embeddings mapping is coming in subsequent phase"})
}

func (s *APIServer) handleModels(c *gin.Context) {
	models := router.GetAllRegistryModels()
	openaiModels := make([]map[string]interface{}, len(models))

	for i, m := range models {
		openaiModels[i] = map[string]interface{}{
			"id":       m.Name,
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": m.Provider,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   openaiModels,
	})
}

func (s *APIServer) handleProviders(c *gin.Context) {
	providers := provider.GetEnabledProviders()
	list := make([]map[string]interface{}, len(providers))

	for i, p := range providers {
		isHealthy, avgLatency, _ := cache.GetProviderHealth(c.Request.Context(), p.Name())
		list[i] = map[string]interface{}{
			"name":        p.Name(),
			"is_healthy":  isHealthy,
			"avg_latency": avgLatency,
		}
	}

	c.JSON(http.StatusOK, list)
}

func (s *APIServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "UP",
		"timestamp":   time.Now().Format(time.RFC3339),
		"environment": "production",
	})
}

func (s *APIServer) handleMetrics(c *gin.Context) {
	telemetry.HTTPHandler().ServeHTTP(c.Writer, c.Request)
}

func (s *APIServer) adminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *APIServer) handleAdminGetProviders(c *gin.Context) {
	mask := func(k string) string {
		if len(k) <= 8 {
			return "****"
		}
		return k[:4] + strings.Repeat("*", len(k)-8) + k[len(k)-4:]
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": []gin.H{
			{
				"name":        "subscription",
				"type":        "cli",
				"enabled":     s.cfg.Providers.Subscription.Enabled,
				"binary_path": s.cfg.Providers.Subscription.BinaryPath,
				"models":      s.cfg.Providers.Subscription.Models,
			},
			{
				"name":        "agy",
				"type":        "cli",
				"enabled":     s.cfg.Providers.Agy.Enabled,
				"binary_path": s.cfg.Providers.Agy.BinaryPath,
				"models":      s.cfg.Providers.Agy.Models,
			},
			{
				"name":    "gemini",
				"type":    "api_key",
				"enabled": s.cfg.Providers.Gemini.Enabled,
				"api_key": mask(s.cfg.Providers.Gemini.APIKey),
				"models":  s.cfg.Providers.Gemini.Models,
			},
			{
				"name":    "openai",
				"type":    "api_key",
				"enabled": s.cfg.Providers.OpenAI.Enabled,
				"api_key": mask(s.cfg.Providers.OpenAI.APIKey),
				"models":  s.cfg.Providers.OpenAI.Models,
			},
			{
				"name":     "ollama",
				"type":     "local",
				"enabled":  s.cfg.Providers.Ollama.Enabled,
				"base_url": s.cfg.Providers.Ollama.BaseURL,
				"models":   s.cfg.Providers.Ollama.Models,
			},
		},
	})
}

type providerUpdateRequest struct {
	Enabled    *bool  `json:"enabled"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	BinaryPath string `json:"binary_path"`
}

func (s *APIServer) handleAdminUpdateProvider(c *gin.Context) {
	name := c.Param("name")
	var req providerUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch name {
	case "gemini":
		if req.APIKey != "" {
			s.cfg.Providers.Gemini.APIKey = req.APIKey
		}
		if req.Enabled != nil {
			s.cfg.Providers.Gemini.Enabled = *req.Enabled
		}
	case "openai":
		if req.APIKey != "" {
			s.cfg.Providers.OpenAI.APIKey = req.APIKey
		}
		if req.Enabled != nil {
			s.cfg.Providers.OpenAI.Enabled = *req.Enabled
		}
	case "ollama":
		if req.BaseURL != "" {
			s.cfg.Providers.Ollama.BaseURL = req.BaseURL
		}
		if req.Enabled != nil {
			s.cfg.Providers.Ollama.Enabled = *req.Enabled
		}
	case "subscription":
		if req.BinaryPath != "" {
			s.cfg.Providers.Subscription.BinaryPath = req.BinaryPath
		}
		if req.Enabled != nil {
			s.cfg.Providers.Subscription.Enabled = *req.Enabled
		}
	case "agy":
		if req.BinaryPath != "" {
			s.cfg.Providers.Agy.BinaryPath = req.BinaryPath
		}
		if req.Enabled != nil {
			s.cfg.Providers.Agy.Enabled = *req.Enabled
		}
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider: " + name})
		return
	}

	provider.InitProviders(s.cfg)
	c.JSON(http.StatusOK, gin.H{"status": "updated", "provider": name})
}

func (s *APIServer) handleAdminTestProvider(c *gin.Context) {
	name := c.Param("name")
	prov, err := provider.GetProvider(name)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "provider not registered: " + name})
		return
	}

	ctx := c.Request.Context()
	resp, err := prov.Chat(ctx, provider.ChatRequest{
		Model: "auto",
		Messages: []provider.ChatMessage{
			{Role: "user", Content: "Reply with exactly: OK"},
		},
	})
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": err.Error()})
		return
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "response": content})
}

func (s *APIServer) handleOrchestrationStatus(c *gin.Context) {
	requestID := c.Param("requestID")
	bb, err := agent.LoadBlackboard(c.Request.Context(), requestID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no blackboard found for request: " + requestID})
		return
	}

	completedTasks := make([]string, 0, len(bb.TaskDrafts))
	for id := range bb.TaskDrafts {
		completedTasks = append(completedTasks, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id":      bb.RequestID,
		"completed_tasks": completedTasks,
		"pending_tasks":   bb.ConversationTurn,
		"activity_log":    bb.Logs,
	})
}

func (s *APIServer) handleRequestLogs(c *gin.Context) {
	limit, offset := 50, 0
	if v := c.Query("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if v := c.Query("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}

	filter := db.TraceFilter{
		Provider:    c.Query("provider"),
		RoutingType: c.Query("routing_type"),
		Complexity:  c.Query("complexity"),
		Status:      c.Query("status"),
		From:        c.Query("from"),
		To:          c.Query("to"),
		Limit:       limit,
		Offset:      offset,
	}

	traces, total, err := db.GetRequestTraces(c.Request.Context(), filter)
	if err != nil {
		logger.Log.Error("Failed to fetch request traces", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"logs":   traces,
	})
}

func (s *APIServer) handleUsageLogs(c *gin.Context) {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &limit); n != 1 || err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
			return
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &offset); n != 1 || err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset parameter"})
			return
		}
	}

	filter := db.UsageLogFilter{
		Provider: c.Query("provider"),
		Model:    c.Query("model"),
		From:     c.Query("from"),
		To:       c.Query("to"),
		Limit:    limit,
		Offset:   offset,
	}

	entries, summary, err := db.GetUsageLogs(c.Request.Context(), filter)
	if err != nil {
		logger.Log.Error("Failed to fetch usage logs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch usage logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": gin.H{
			"total_prompt_tokens":     summary.TotalPromptTokens,
			"total_completion_tokens": summary.TotalCompletionTokens,
			"total_tokens":            summary.TotalPromptTokens + summary.TotalCompletionTokens,
			"total_cost":              summary.TotalCost,
			"request_count":           summary.RequestCount,
		},
		"logs":   entries,
		"limit":  limit,
		"offset": offset,
	})
}
