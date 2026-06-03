package cache

import (
	"context"
	"testing"
	"time"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
)

func setupCache(t *testing.T) context.Context {
	t.Helper()
	if logger.Log == nil {
		logger.InitLogger("development")
	}
	if err := InitCache(&config.CacheConfig{CacheTTLSeconds: 3600}); err != nil {
		t.Fatalf("InitCache failed: %v", err)
	}
	return context.Background()
}

func TestResponseCacheHitAndMiss(t *testing.T) {
	ctx := setupCache(t)

	key := GenerateCacheKey("hello", "gpt-4o", false)
	if key == "" {
		t.Fatal("expected non-empty cache key")
	}

	// Miss.
	got, err := GetResponseCache(ctx, key)
	if err != nil {
		t.Fatalf("GetResponseCache miss errored: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil on miss, got %q", string(got))
	}

	// Hit.
	if err := SetResponseCache(ctx, key, []byte("payload")); err != nil {
		t.Fatalf("SetResponseCache failed: %v", err)
	}
	got, err = GetResponseCache(ctx, key)
	if err != nil {
		t.Fatalf("GetResponseCache hit errored: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("expected 'payload', got %q", string(got))
	}
}

func TestSetBytesTTLExpiry(t *testing.T) {
	ctx := setupCache(t)

	if err := SetBytes(ctx, "k:short", []byte("v"), 40*time.Millisecond); err != nil {
		t.Fatalf("SetBytes failed: %v", err)
	}
	if got, _ := GetBytes(ctx, "k:short"); string(got) != "v" {
		t.Fatalf("expected value before expiry, got %q", string(got))
	}
	time.Sleep(80 * time.Millisecond)
	if got, _ := GetBytes(ctx, "k:short"); got != nil {
		t.Errorf("expected nil after TTL expiry, got %q", string(got))
	}
}

func TestRateLimitCheck(t *testing.T) {
	ctx := setupCache(t)

	// Unlimited when limit <= 0.
	allowed, remaining, err := RateLimitCheck(ctx, "key-unlimited", 0)
	if err != nil || !allowed || remaining != 9999 {
		t.Errorf("unlimited path wrong: allowed=%v remaining=%d err=%v", allowed, remaining, err)
	}

	const apiKey = "key-limited"
	const limit = 3
	for i := 1; i <= limit; i++ {
		allowed, _, err := RateLimitCheck(ctx, apiKey, limit)
		if err != nil {
			t.Fatalf("RateLimitCheck errored: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed", i)
		}
	}
	// Next one trips the limit.
	allowed, _, err = RateLimitCheck(ctx, apiKey, limit)
	if err != nil {
		t.Fatalf("RateLimitCheck errored: %v", err)
	}
	if allowed {
		t.Error("expected request over limit to be denied")
	}
}

func TestBudgetTracking(t *testing.T) {
	ctx := setupCache(t)

	const apiKey = "budget-key"
	if _, _, err := TrackBudgetUsage(ctx, apiKey, 1.5); err != nil {
		t.Fatalf("TrackBudgetUsage failed: %v", err)
	}
	daily, monthly, err := TrackBudgetUsage(ctx, apiKey, 2.5)
	if err != nil {
		t.Fatalf("TrackBudgetUsage failed: %v", err)
	}
	if daily != 4.0 || monthly != 4.0 {
		t.Errorf("expected accumulated 4.0, got daily=%v monthly=%v", daily, monthly)
	}

	gotDaily, gotMonthly, err := GetCachedBudgetUsage(ctx, apiKey)
	if err != nil {
		t.Fatalf("GetCachedBudgetUsage failed: %v", err)
	}
	if gotDaily != 4.0 || gotMonthly != 4.0 {
		t.Errorf("expected 4.0/4.0, got %v/%v", gotDaily, gotMonthly)
	}
}

func TestSessionHistoryOrder(t *testing.T) {
	ctx := setupCache(t)

	const sid = "sess-1"
	msgs := []Message{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "user", Content: "three"},
	}
	for _, m := range msgs {
		if err := SaveSessionMessage(ctx, sid, m); err != nil {
			t.Fatalf("SaveSessionMessage failed: %v", err)
		}
	}

	got, err := GetSessionHistory(ctx, sid)
	if err != nil {
		t.Fatalf("GetSessionHistory failed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
	if got[0].Content != "one" || got[2].Content != "three" {
		t.Errorf("messages out of order: %+v", got)
	}
}

func TestProviderHealth(t *testing.T) {
	ctx := setupCache(t)

	if err := SetProviderStatus(ctx, "openai", true, 100); err != nil {
		t.Fatalf("SetProviderStatus failed: %v", err)
	}
	if err := SetProviderStatus(ctx, "openai", true, 200); err != nil {
		t.Fatalf("SetProviderStatus failed: %v", err)
	}

	healthy, avg, err := GetProviderHealth(ctx, "openai")
	if err != nil {
		t.Fatalf("GetProviderHealth failed: %v", err)
	}
	if !healthy {
		t.Error("expected provider to be healthy")
	}
	if avg != 150 {
		t.Errorf("expected avg latency 150, got %d", avg)
	}

	// Unknown provider defaults to healthy with zero latency.
	healthy, avg, err = GetProviderHealth(ctx, "unknown")
	if err != nil {
		t.Fatalf("GetProviderHealth(unknown) failed: %v", err)
	}
	if !healthy || avg != 0 {
		t.Errorf("expected healthy/0 for unknown provider, got %v/%d", healthy, avg)
	}

	// Mark unhealthy.
	if err := SetProviderStatus(ctx, "gemini", false, 10); err != nil {
		t.Fatalf("SetProviderStatus failed: %v", err)
	}
	healthy, _, _ = GetProviderHealth(ctx, "gemini")
	if healthy {
		t.Error("expected gemini to be unhealthy")
	}
}

func TestListPushBoundedAndRange(t *testing.T) {
	ctx := setupCache(t)

	const key = "metrics:list"
	for i := 0; i < 10; i++ {
		if err := ListPush(ctx, key, []byte{byte('0' + i)}, 5); err != nil {
			t.Fatalf("ListPush failed: %v", err)
		}
	}

	all, err := ListRange(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ListRange failed: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected bounded length 5, got %d", len(all))
	}
	// Newest value pushed last ('9') should be at the head.
	if all[0] != "9" {
		t.Errorf("expected newest '9' at head, got %q", all[0])
	}
}
