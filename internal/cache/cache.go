package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

var Client *redis.Client
var cacheTTL time.Duration

func InitRedis(cfg *config.RedisConfig) error {
	Client = redis.NewClient(&redis.Options{
		Addr:     cfg.Host,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping redis: %w", err)
	}

	cacheTTL = time.Duration(cfg.CacheTTLSeconds) * time.Second
	logger.Log.Info("Successfully connected to Redis", zap.String("host", cfg.Host))
	return nil
}

func GenerateCacheKey(prompt string, model string, stream bool) string {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s:%s:%t", prompt, model, stream)))
	return "cache:" + hex.EncodeToString(hasher.Sum(nil))
}

func GetResponseCache(ctx context.Context, key string) ([]byte, error) {
	val, err := Client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		logger.Log.Error("Redis cache get failure", zap.Error(err), zap.String("key", key))
		return nil, err
	}
	return val, nil
}

func SetResponseCache(ctx context.Context, key string, data []byte) error {
	err := Client.Set(ctx, key, data, cacheTTL).Err()
	if err != nil {
		logger.Log.Error("Redis cache set failure", zap.Error(err), zap.String("key", key))
		return err
	}
	return nil
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func SaveSessionMessage(ctx context.Context, sessionID string, msg Message) error {
	key := "session:" + sessionID
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	pipe := Client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.Expire(ctx, key, 24*time.Hour)
	_, err = pipe.Exec(ctx)
	if err != nil {
		logger.Log.Error("Failed to save session message", zap.Error(err), zap.String("session_id", sessionID))
	}
	return err
}

func GetSessionHistory(ctx context.Context, sessionID string) ([]Message, error) {
	key := "session:" + sessionID
	vals, err := Client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	messages := make([]Message, len(vals))
	for i, val := range vals {
		var msg Message
		if err := json.Unmarshal([]byte(val), &msg); err != nil {
			return nil, err
		}
		messages[i] = msg
	}
	return messages, nil
}

func RateLimitCheck(ctx context.Context, apiKey string, limit int) (bool, int, error) {
	if limit <= 0 {
		return true, 9999, nil
	}

	now := time.Now()
	minuteStr := now.Format("2006-01-02 15:04")
	key := fmt.Sprintf("ratelimit:%s:%s", apiKey, minuteStr)

	pipe := Client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 2*time.Minute)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, err
	}

	count := int(incr.Val())
	if count > limit {
		return false, 0, nil
	}

	return true, limit - count, nil
}

func TrackBudgetUsage(ctx context.Context, apiKey string, cost float64) (float64, float64, error) {
	now := time.Now()
	dailyKey := fmt.Sprintf("budget:daily:%s:%s", apiKey, now.Format("2006-01-02"))
	monthlyKey := fmt.Sprintf("budget:monthly:%s:%s", apiKey, now.Format("2006-01"))

	pipe := Client.Pipeline()
	dailyInc := pipe.IncrByFloat(ctx, dailyKey, cost)
	pipe.Expire(ctx, dailyKey, 25*time.Hour)
	monthlyInc := pipe.IncrByFloat(ctx, monthlyKey, cost)
	pipe.Expire(ctx, monthlyKey, 32*24*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, 0, err
	}

	return dailyInc.Val(), monthlyInc.Val(), nil
}

func GetCachedBudgetUsage(ctx context.Context, apiKey string) (float64, float64, error) {
	now := time.Now()
	dailyKey := fmt.Sprintf("budget:daily:%s:%s", apiKey, now.Format("2006-01-02"))
	monthlyKey := fmt.Sprintf("budget:monthly:%s:%s", apiKey, now.Format("2006-01"))

	pipe := Client.Pipeline()
	dailyVal := pipe.Get(ctx, dailyKey)
	monthlyVal := pipe.Get(ctx, monthlyKey)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return 0, 0, err
	}

	dCost, _ := dailyVal.Float64()
	mCost, _ := monthlyVal.Float64()

	return dCost, mCost, nil
}

func SetProviderStatus(ctx context.Context, provider string, isHealthy bool, latencyMs int64) error {
	pipe := Client.Pipeline()
	statusKey := "health:status:" + provider
	latencyKey := "health:latency:" + provider

	status := "healthy"
	if !isHealthy {
		status = "unhealthy"
	}

	pipe.Set(ctx, statusKey, status, 24*time.Hour)
	pipe.LPush(ctx, latencyKey, latencyMs)
	pipe.LTrim(ctx, latencyKey, 0, 99)
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Log.Error("Failed to update provider status in Redis", zap.Error(err), zap.String("provider", provider))
	}
	return err
}

func GetProviderHealth(ctx context.Context, provider string) (bool, int64, error) {
	statusKey := "health:status:" + provider
	latencyKey := "health:latency:" + provider

	status, err := Client.Get(ctx, statusKey).Result()
	if err != nil && err != redis.Nil {
		return false, 0, err
	}

	isHealthy := status != "unhealthy"

	latencies, err := Client.LRange(ctx, latencyKey, 0, 9).Result()
	var avgLatency int64 = 0
	if err == nil && len(latencies) > 0 {
		var sum int64 = 0
		var validCount int64 = 0
		for _, lStr := range latencies {
			var l int64
			if _, err := fmt.Sscanf(lStr, "%d", &l); err == nil {
				sum += l
				validCount++
			}
		}
		if validCount > 0 {
			avgLatency = sum / validCount
		}
	}

	return isHealthy, avgLatency, nil
}
