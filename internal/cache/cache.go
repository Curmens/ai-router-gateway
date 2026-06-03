package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

// item is a single cache entry. It holds either a byte value (data) or an
// ordered list (list), distinguished by isList. A zero expiresAt means the
// entry never expires.
type item struct {
	data      []byte
	list      [][]byte
	expiresAt time.Time
	isList    bool
}

// memStore is a thread-safe in-memory key/value + list store with TTLs. The
// single mutex makes read-modify-write operations (incr/incrByFloat) atomic,
// replacing the atomicity Redis INCR/INCRBYFLOAT provided.
type memStore struct {
	mu sync.Mutex
	m  map[string]*item
}

func newMemStore() *memStore {
	return &memStore{m: make(map[string]*item)}
}

// liveLocked returns the entry for key if present and not expired, deleting it
// lazily on expiry. Caller must hold the mutex.
func (s *memStore) liveLocked(key string) (*item, bool) {
	it, ok := s.m[key]
	if !ok {
		return nil, false
	}
	if !it.expiresAt.IsZero() && time.Now().After(it.expiresAt) {
		delete(s.m, key)
		return nil, false
	}
	return it, true
}

func (s *memStore) set(key string, data []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := &item{data: data}
	if ttl > 0 {
		it.expiresAt = time.Now().Add(ttl)
	}
	s.m[key] = it
}

func (s *memStore) get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.liveLocked(key)
	if !ok || it.isList {
		return nil, false
	}
	return it.data, true
}

func (s *memStore) incr(key string, ttl time.Duration) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	if it, ok := s.liveLocked(key); ok && !it.isList {
		n, _ = strconv.ParseInt(string(it.data), 10, 64)
	}
	n++
	it := &item{data: []byte(strconv.FormatInt(n, 10))}
	if ttl > 0 {
		it.expiresAt = time.Now().Add(ttl)
	}
	s.m[key] = it
	return n
}

func (s *memStore) incrByFloat(key string, delta float64, ttl time.Duration) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var f float64
	if it, ok := s.liveLocked(key); ok && !it.isList {
		f, _ = strconv.ParseFloat(string(it.data), 64)
	}
	f += delta
	it := &item{data: []byte(strconv.FormatFloat(f, 'f', -1, 64))}
	if ttl > 0 {
		it.expiresAt = time.Now().Add(ttl)
	}
	s.m[key] = it
	return f
}

func (s *memStore) lpush(key string, val []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.liveLocked(key)
	if !ok || !it.isList {
		it = &item{isList: true}
		s.m[key] = it
	}
	it.list = append([][]byte{val}, it.list...)
}

func (s *memStore) rpush(key string, val []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.liveLocked(key)
	if !ok || !it.isList {
		it = &item{isList: true}
		s.m[key] = it
	}
	it.list = append(it.list, val)
}

func (s *memStore) ltrim(key string, start, stop int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.liveLocked(key)
	if !ok || !it.isList {
		return
	}
	lo, hi, empty := normalizeRange(len(it.list), start, stop)
	if empty {
		it.list = nil
		return
	}
	it.list = append([][]byte{}, it.list[lo:hi+1]...)
}

func (s *memStore) lrange(key string, start, stop int) [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.liveLocked(key)
	if !ok || !it.isList {
		return nil
	}
	lo, hi, empty := normalizeRange(len(it.list), start, stop)
	if empty {
		return nil
	}
	return append([][]byte{}, it.list[lo:hi+1]...)
}

func (s *memStore) expire(key string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.liveLocked(key); ok {
		if ttl > 0 {
			it.expiresAt = time.Now().Add(ttl)
		} else {
			it.expiresAt = time.Time{}
		}
	}
}

func (s *memStore) sweepExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, it := range s.m {
		if !it.expiresAt.IsZero() && now.After(it.expiresAt) {
			delete(s.m, k)
		}
	}
}

// normalizeRange resolves Redis-style (possibly negative) indices into a
// concrete [lo, hi] inclusive slice range. empty is true when the range
// selects nothing.
func normalizeRange(length, start, stop int) (lo int, hi int, empty bool) {
	if length == 0 {
		return 0, 0, true
	}
	if start < 0 {
		start = length + start
		if start < 0 {
			start = 0
		}
	}
	if stop < 0 {
		stop = length + stop
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop || start >= length {
		return 0, 0, true
	}
	return start, stop, false
}

var (
	store       *memStore
	cacheTTL    time.Duration
	sweeperOnce sync.Once
)

// InitCache builds the in-memory store and starts a background sweeper that
// drops expired keys. It never fails, but keeps an error return for a stable
// initialization signature.
func InitCache(cfg *config.CacheConfig) error {
	store = newMemStore()
	cacheTTL = time.Duration(cfg.CacheTTLSeconds) * time.Second
	sweeperOnce.Do(func() {
		go sweepLoop(60 * time.Second)
	})
	logger.Log.Info("Initialized in-memory cache", zap.Int("cache_ttl_seconds", cfg.CacheTTLSeconds))
	return nil
}

func sweepLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if store != nil {
			store.sweepExpired()
		}
	}
}

func GenerateCacheKey(prompt string, model string, stream bool) string {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s:%s:%t", prompt, model, stream)))
	return "cache:" + hex.EncodeToString(hasher.Sum(nil))
}

func GetResponseCache(ctx context.Context, key string) ([]byte, error) {
	if v, ok := store.get(key); ok {
		return v, nil
	}
	return nil, nil
}

func SetResponseCache(ctx context.Context, key string, data []byte) error {
	store.set(key, data, cacheTTL)
	return nil
}

// SetBytes stores an arbitrary value with a caller-supplied TTL (0 = no expiry).
func SetBytes(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	store.set(key, data, ttl)
	return nil
}

// GetBytes returns the value for key, or (nil, nil) on a miss.
func GetBytes(ctx context.Context, key string) ([]byte, error) {
	if v, ok := store.get(key); ok {
		return v, nil
	}
	return nil, nil
}

// ListPush prepends value to the list at key and trims it to maxLen items
// (newest first), mirroring the old Redis LPUSH + LTRIM pattern.
func ListPush(ctx context.Context, key string, value []byte, maxLen int) error {
	store.lpush(key, value)
	if maxLen > 0 {
		store.ltrim(key, 0, maxLen-1)
	}
	return nil
}

// ListRange returns the list slice [start, stop] (Redis-style indices) as strings.
func ListRange(ctx context.Context, key string, start, stop int) ([]string, error) {
	vals := store.lrange(key, start, stop)
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = string(v)
	}
	return out, nil
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
	store.rpush(key, data)
	store.expire(key, 24*time.Hour)
	return nil
}

func GetSessionHistory(ctx context.Context, sessionID string) ([]Message, error) {
	key := "session:" + sessionID
	vals := store.lrange(key, 0, -1)
	if len(vals) == 0 {
		return nil, nil
	}

	messages := make([]Message, len(vals))
	for i, val := range vals {
		var msg Message
		if err := json.Unmarshal(val, &msg); err != nil {
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

	count := int(store.incr(key, 2*time.Minute))
	if count > limit {
		return false, 0, nil
	}

	return true, limit - count, nil
}

func TrackBudgetUsage(ctx context.Context, apiKey string, cost float64) (float64, float64, error) {
	now := time.Now()
	dailyKey := fmt.Sprintf("budget:daily:%s:%s", apiKey, now.Format("2006-01-02"))
	monthlyKey := fmt.Sprintf("budget:monthly:%s:%s", apiKey, now.Format("2006-01"))

	daily := store.incrByFloat(dailyKey, cost, 25*time.Hour)
	monthly := store.incrByFloat(monthlyKey, cost, 32*24*time.Hour)
	return daily, monthly, nil
}

func GetCachedBudgetUsage(ctx context.Context, apiKey string) (float64, float64, error) {
	now := time.Now()
	dailyKey := fmt.Sprintf("budget:daily:%s:%s", apiKey, now.Format("2006-01-02"))
	monthlyKey := fmt.Sprintf("budget:monthly:%s:%s", apiKey, now.Format("2006-01"))

	return getFloat(dailyKey), getFloat(monthlyKey), nil
}

func getFloat(key string) float64 {
	if v, ok := store.get(key); ok {
		f, _ := strconv.ParseFloat(string(v), 64)
		return f
	}
	return 0
}

func SetProviderStatus(ctx context.Context, provider string, isHealthy bool, latencyMs int64) error {
	statusKey := "health:status:" + provider
	latencyKey := "health:latency:" + provider

	status := "healthy"
	if !isHealthy {
		status = "unhealthy"
	}

	store.set(statusKey, []byte(status), 24*time.Hour)
	store.lpush(latencyKey, []byte(strconv.FormatInt(latencyMs, 10)))
	store.ltrim(latencyKey, 0, 99)
	return nil
}

func GetProviderHealth(ctx context.Context, provider string) (bool, int64, error) {
	statusKey := "health:status:" + provider
	latencyKey := "health:latency:" + provider

	isHealthy := true
	if v, ok := store.get(statusKey); ok && string(v) == "unhealthy" {
		isHealthy = false
	}

	latencies := store.lrange(latencyKey, 0, 9)
	var avgLatency int64 = 0
	if len(latencies) > 0 {
		var sum int64 = 0
		var validCount int64 = 0
		for _, l := range latencies {
			var v int64
			if _, err := fmt.Sscanf(string(l), "%d", &v); err == nil {
				sum += v
				validCount++
			}
		}
		if validCount > 0 {
			avgLatency = sum / validCount
		}
	}

	return isHealthy, avgLatency, nil
}
