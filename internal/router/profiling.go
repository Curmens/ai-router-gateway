package router

import (
	"context"
	"encoding/json"

	"github.com/user1024/auto-router/internal/cache"
)

func LogCallMetrics(ctx context.Context, provider string, latencyMs int64, hasError bool, responseText string, isJSONRequested bool) {
	cache.PushMetric("health:latency:"+provider, float64(latencyMs))

	errVal := 0.0
	if hasError {
		errVal = 1.0
	}
	cache.PushMetric("profiling:errors:"+provider, errVal)

	if isJSONRequested && !hasError {
		var v interface{}
		syntaxVal := 1.0
		if json.Unmarshal([]byte(responseText), &v) != nil {
			syntaxVal = 0.0
		}
		cache.PushMetric("profiling:syntax:"+provider, syntaxVal)
	}
}

func GetProviderPerformanceScore(ctx context.Context, provider string) float64 {
	latencies := cache.GetMetrics("health:latency:" + provider)
	avgLatency := 500.0
	if len(latencies) > 0 {
		var sum float64
		for _, l := range latencies {
			sum += l
		}
		avgLatency = sum / float64(len(latencies))
	}

	errors := cache.GetMetrics("profiling:errors:" + provider)
	var errorRate float64
	if len(errors) > 0 {
		var sum float64
		for _, e := range errors {
			sum += e
		}
		errorRate = sum / float64(len(errors))
	}

	syntax := cache.GetMetrics("profiling:syntax:" + provider)
	avgSyntax := 1.0
	if len(syntax) > 0 {
		var sum float64
		for _, s := range syntax {
			sum += s
		}
		avgSyntax = sum / float64(len(syntax))
	}

	latencyScore := 1.0 / (1.0 + (avgLatency / 1000.0))
	successScore := 1.0 - errorRate
	return (0.4 * latencyScore) + (0.3 * successScore) + (0.3 * avgSyntax)
}
