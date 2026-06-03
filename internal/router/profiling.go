package router

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/user1024/auto-router/internal/cache"
)

const profilingMaxLen = 20

func LogCallMetrics(ctx context.Context, provider string, latencyMs int64, hasError bool, responseText string, isJSONRequested bool) {
	latencyKey := "health:latency:" + provider
	_ = cache.ListPush(ctx, latencyKey, []byte(strconv.FormatInt(latencyMs, 10)), profilingMaxLen)

	errorKey := "profiling:errors:" + provider
	errVal := "0"
	if hasError {
		errVal = "1"
	}
	_ = cache.ListPush(ctx, errorKey, []byte(errVal), profilingMaxLen)

	if isJSONRequested && !hasError {
		syntaxKey := "profiling:syntax:" + provider
		var isValid interface{}
		isJSONValid := json.Unmarshal([]byte(responseText), &isValid) == nil

		syntaxVal := "1"
		if !isJSONValid {
			syntaxVal = "0"
		}
		_ = cache.ListPush(ctx, syntaxKey, []byte(syntaxVal), profilingMaxLen)
	}
}

func GetProviderPerformanceScore(ctx context.Context, provider string) float64 {
	latencyKey := "health:latency:" + provider
	errorKey := "profiling:errors:" + provider
	syntaxKey := "profiling:syntax:" + provider

	latencies, err := cache.ListRange(ctx, latencyKey, 0, 19)
	var avgLatency float64 = 500.0
	if err == nil && len(latencies) > 0 {
		var sum float64 = 0
		for _, lStr := range latencies {
			if l, err := strconv.ParseFloat(lStr, 64); err == nil {
				sum += l
			}
		}
		avgLatency = sum / float64(len(latencies))
	}

	errorsList, err := cache.ListRange(ctx, errorKey, 0, 19)
	var errorRate float64 = 0.0
	if err == nil && len(errorsList) > 0 {
		var errSum float64 = 0
		for _, eStr := range errorsList {
			if e, err := strconv.ParseFloat(eStr, 64); err == nil {
				errSum += e
			}
		}
		errorRate = errSum / float64(len(errorsList))
	}

	syntaxList, err := cache.ListRange(ctx, syntaxKey, 0, 19)
	var avgSyntax float64 = 1.0
	if err == nil && len(syntaxList) > 0 {
		var synSum float64 = 0
		for _, sStr := range syntaxList {
			if s, err := strconv.ParseFloat(sStr, 64); err == nil {
				synSum += s
			}
		}
		avgSyntax = synSum / float64(len(syntaxList))
	}

	latencyScore := 1.0 / (1.0 + (avgLatency / 1000.0))
	successScore := 1.0 - errorRate

	return (0.4 * latencyScore) + (0.3 * successScore) + (0.3 * avgSyntax)
}
