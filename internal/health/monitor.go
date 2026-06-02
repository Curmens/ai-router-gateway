package health

import (
	"context"
	"time"

	"github.com/user1024/auto-router/internal/cache"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"go.uber.org/zap"
)

type Monitor struct {
	stopChan chan struct{}
}

func NewMonitor() *Monitor {
	return &Monitor{
		stopChan: make(chan struct{}),
	}
}

func (m *Monitor) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		m.checkAllProviders()

		for {
			select {
			case <-ticker.C:
				m.checkAllProviders()
			case <-m.stopChan:
				return
			}
		}
	}()
}

func (m *Monitor) Stop() {
	close(m.stopChan)
}

func (m *Monitor) checkAllProviders() {
	providers := provider.GetEnabledProviders()
	for _, p := range providers {
		go func(prov provider.ChatProvider) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			start := time.Now()
			_, err := prov.Models(ctx)
			latency := time.Since(start)

			isHealthy := err == nil
			latencyMs := latency.Milliseconds()

			if err != nil {
				logger.Log.Warn("Provider health check failed", zap.String("provider", prov.Name()), zap.Error(err))
			}

			_ = cache.SetProviderStatus(ctx, prov.Name(), isHealthy, latencyMs)
		}(p)
	}
}
