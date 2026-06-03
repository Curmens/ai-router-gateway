package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

const defaultCacheTTL = time.Hour

// Message is a chat message stored in session history.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

type rateEntry struct {
	count     int
	expiresAt time.Time
}

type budgetEntry struct {
	dailyUsage   float64
	monthlyUsage float64
	day          string
	month        string
}

var cacheTTL = defaultCacheTTL

var (
	respMu    sync.RWMutex
	respCache = make(map[string]cacheEntry)

	sessMu   sync.RWMutex
	sessions = make(map[string][]Message)

	rateMu    sync.Mutex
	rateStore = make(map[string]rateEntry)

	budgetMu sync.Mutex
	budgets  = make(map[string]*budgetEntry)

	statusMu  sync.RWMutex
	statusMap = make(map[string]bool)

	ringMu      sync.RWMutex
	ringBuffers = make(map[string][]float64)

	blobMu    sync.RWMutex
	blobStore = make(map[string]cacheEntry)
)

func Init() {
	logger.Log.Info("In-memory cache initialized", zap.Duration("ttl", cacheTTL))
}

func GenerateCacheKey(prompt, model string, stream bool) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%t", prompt, model, stream)))
	return "cache:" + hex.EncodeToString(h.Sum(nil))
}

func GetResponseCache(ctx context.Context, key string) ([]byte, error) {
	respMu.RLock()
	entry, ok := respCache[key]
	respMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, nil
	}
	return entry.data, nil
}

func SetResponseCache(ctx context.Context, key string, data []byte) error {
	respMu.Lock()
	respCache[key] = cacheEntry{data: data, expiresAt: time.Now().Add(cacheTTL)}
	respMu.Unlock()
	return nil
}

func SaveSessionMessage(ctx context.Context, sessionID string, msg Message) error {
	sessMu.Lock()
	sessions[sessionID] = append(sessions[sessionID], msg)
	sessMu.Unlock()
	return nil
}

func GetSessionHistory(ctx context.Context, sessionID string) ([]Message, error) {
	sessMu.RLock()
	msgs := make([]Message, len(sessions[sessionID]))
	copy(msgs, sessions[sessionID])
	sessMu.RUnlock()
	return msgs, nil
}

func RateLimitCheck(ctx context.Context, apiKey string, limit int) (bool, int, error) {
	if limit <= 0 {
		return true, 9999, nil
	}

	now := time.Now()
	key := fmt.Sprintf("ratelimit:%s:%s", apiKey, now.Format("2006-01-02 15:04"))

	rateMu.Lock()
	entry, ok := rateStore[key]
	if !ok || time.Now().After(entry.expiresAt) {
		entry = rateEntry{count: 0, expiresAt: now.Add(2 * time.Minute)}
	}
	entry.count++
	rateStore[key] = entry
	rateMu.Unlock()

	if entry.count > limit {
		return false, 0, nil
	}
	return true, limit - entry.count, nil
}

func TrackBudgetUsage(ctx context.Context, apiKey string, cost float64) (float64, float64, error) {
	now := time.Now()
	day := now.Format("2006-01-02")
	month := now.Format("2006-01")

	budgetMu.Lock()
	b, ok := budgets[apiKey]
	if !ok {
		b = &budgetEntry{day: day, month: month}
		budgets[apiKey] = b
	}
	if b.day != day {
		b.dailyUsage = 0
		b.day = day
	}
	if b.month != month {
		b.monthlyUsage = 0
		b.month = month
	}
	b.dailyUsage += cost
	b.monthlyUsage += cost
	daily, monthly := b.dailyUsage, b.monthlyUsage
	budgetMu.Unlock()

	return daily, monthly, nil
}

func GetCachedBudgetUsage(ctx context.Context, apiKey string) (float64, float64, error) {
	now := time.Now()
	day := now.Format("2006-01-02")
	month := now.Format("2006-01")

	budgetMu.Lock()
	b, ok := budgets[apiKey]
	budgetMu.Unlock()

	if !ok {
		return 0, 0, nil
	}
	var daily, monthly float64
	if b.day == day {
		daily = b.dailyUsage
	}
	if b.month == month {
		monthly = b.monthlyUsage
	}
	return daily, monthly, nil
}

func SetProviderStatus(ctx context.Context, provider string, isHealthy bool, latencyMs int64) error {
	statusMu.Lock()
	statusMap[provider] = isHealthy
	statusMu.Unlock()
	PushMetric("health:latency:"+provider, float64(latencyMs))
	return nil
}

func GetProviderHealth(ctx context.Context, provider string) (bool, int64, error) {
	statusMu.RLock()
	isHealthy, ok := statusMap[provider]
	statusMu.RUnlock()
	if !ok {
		isHealthy = true
	}

	lats := GetMetrics("health:latency:" + provider)
	if len(lats) > 10 {
		lats = lats[len(lats)-10:]
	}
	var sum int64
	for _, l := range lats {
		sum += int64(l)
	}
	var avg int64
	if len(lats) > 0 {
		avg = sum / int64(len(lats))
	}
	return isHealthy, avg, nil
}

// PushMetric appends a value to a named ring buffer (max 20 entries).
// Used by profiling and health monitoring.
func PushMetric(key string, value float64) {
	ringMu.Lock()
	buf := ringBuffers[key]
	buf = append(buf, value)
	if len(buf) > 20 {
		buf = buf[len(buf)-20:]
	}
	ringBuffers[key] = buf
	ringMu.Unlock()
}

// GetMetrics returns a copy of the named ring buffer.
func GetMetrics(key string) []float64 {
	ringMu.RLock()
	src := ringBuffers[key]
	result := make([]float64, len(src))
	copy(result, src)
	ringMu.RUnlock()
	return result
}

// StoreBlob stores arbitrary bytes under a key with a TTL.
// Used by the blackboard agent system.
func StoreBlob(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	blobMu.Lock()
	blobStore[key] = cacheEntry{data: data, expiresAt: time.Now().Add(ttl)}
	blobMu.Unlock()
	return nil
}

// LoadBlob retrieves bytes stored by StoreBlob.
func LoadBlob(ctx context.Context, key string) ([]byte, error) {
	blobMu.RLock()
	entry, ok := blobStore[key]
	blobMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return entry.data, nil
}
