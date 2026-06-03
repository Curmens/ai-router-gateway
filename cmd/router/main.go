package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/user1024/auto-router/internal/agent"
	"github.com/user1024/auto-router/internal/api"
	"github.com/user1024/auto-router/internal/cache"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/db"
	"github.com/user1024/auto-router/internal/graph"
	"github.com/user1024/auto-router/internal/health"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"github.com/user1024/auto-router/internal/router"
	"github.com/user1024/auto-router/internal/telemetry"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	logger.InitLogger("development")
	defer func() {
		if logger.Log != nil {
			_ = logger.Log.Sync()
		}
	}()

	logger.Log.Info("Starting AI Router gateway...")

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Log.Fatal("Failed to load configurations", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.InitDB(ctx, &cfg.Database); err != nil {
		logger.Log.Fatal("Failed to initialize database", zap.Error(err))
	}

	cache.Init()

	// Initialize Provider layers
	provider.InitProviders(cfg)
	router.InitRegistry(cfg)
	router.InitCircuitBreakers(cfg)
	telemetry.InitTelemetry(&cfg.Telemetry)

	// Inject default budgets into PostgreSQL to boot test API Keys
	seedDefaultBudgets(ctx)

	// Launch Health Monitor background supervisor (checks every 30s)
	monitor := health.NewMonitor()
	monitor.Start(30 * time.Second)
	defer monitor.Stop()

	// Initialize Graph Project Resolver
	defaultPath := "/home/user1024/Projects/auto-router"
	if cfg.Routing.GraphPath != "" {
		// Use parent of graphify-out/graph.json if custom GraphPath set
		defaultPath = filepath.Dir(filepath.Dir(cfg.Routing.GraphPath))
	}
	graph.InitResolver(defaultPath)
	if err := graph.ActiveResolver.ScanProjects("/home/user1024/Projects"); err != nil {
		logger.Log.Warn("Failed to scan project directories", zap.Error(err))
	}

	orch := agent.NewOrchestrator(cfg)

	engine := router.NewRoutingEngine(cfg)
	failover := router.NewFailoverManager(cfg, engine)
	server := api.NewAPIServer(cfg, failover, engine, orch)

	// Spawn HTTP Server in background
	go func() {
		if err := server.Start(); err != nil {
			logger.Log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutdown signal received, terminating gateway services gracefully...")
}

func seedDefaultBudgets(ctx context.Context) {
	// Seed budget limits for the default API Keys defined in configs/config.example.yaml
	_ = db.UpsertBudgetLimits(ctx, "sk-router-admin-12345", "admin", 1000.0, 5000.0)
	_ = db.UpsertBudgetLimits(ctx, "sk-router-dev-67890", "developer", 100.0, 500.0)
	_ = db.UpsertBudgetLimits(ctx, "sk-router-user-54321", "user", 10.0, 50.0)
	logger.Log.Info("Default budget limits seeded successfully")
}
