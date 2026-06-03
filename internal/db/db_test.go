package db

import (
	"context"
	"testing"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
)

func setupTestDB(t *testing.T) context.Context {
	t.Helper()
	if logger.Log == nil {
		logger.InitLogger("development")
	}
	ctx := context.Background()
	if err := InitDB(ctx, &config.StorageConfig{Path: t.TempDir()}); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		if DB != nil {
			_ = DB.Close()
			DB = nil
		}
	})
	return ctx
}

func TestSaveRequestAndGetUsageLogs(t *testing.T) {
	ctx := setupTestDB(t)

	reqs := []DBRequest{
		{ID: "r1", Provider: "openai", Model: "gpt-4o", PromptTokens: 10, CompletionTokens: 20, Cost: 0.5, LatencyMs: 100, Status: 200, Prompt: "hi"},
		{ID: "r2", Provider: "openai", Model: "gpt-4o", PromptTokens: 5, CompletionTokens: 5, Cost: 0.25, LatencyMs: 50, Status: 200, Prompt: "yo"},
		{ID: "r3", Provider: "gemini", Model: "gemini-2.5-pro", PromptTokens: 1, CompletionTokens: 1, Cost: 1.0, LatencyMs: 10, Status: 500, ErrorMessage: "boom", Prompt: "fail"},
	}
	for _, r := range reqs {
		if err := SaveRequest(ctx, r); err != nil {
			t.Fatalf("SaveRequest(%s) failed: %v", r.ID, err)
		}
	}

	// No filter: only status=200 rows are counted.
	entries, summary, err := GetUsageLogs(ctx, UsageLogFilter{})
	if err != nil {
		t.Fatalf("GetUsageLogs failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if summary.RequestCount != 2 {
		t.Errorf("expected RequestCount=2, got %d", summary.RequestCount)
	}
	if summary.TotalPromptTokens != 15 {
		t.Errorf("expected TotalPromptTokens=15, got %d", summary.TotalPromptTokens)
	}
	if summary.TotalCompletionTokens != 25 {
		t.Errorf("expected TotalCompletionTokens=25, got %d", summary.TotalCompletionTokens)
	}
	if summary.TotalCost != 0.75 {
		t.Errorf("expected TotalCost=0.75, got %v", summary.TotalCost)
	}

	// Filter by a provider that has no successful rows.
	gemEntries, gemSummary, err := GetUsageLogs(ctx, UsageLogFilter{Provider: "gemini"})
	if err != nil {
		t.Fatalf("GetUsageLogs(gemini) failed: %v", err)
	}
	if len(gemEntries) != 0 || gemSummary.RequestCount != 0 {
		t.Errorf("expected no gemini entries, got %d (count=%d)", len(gemEntries), gemSummary.RequestCount)
	}
}

func TestSaveAuditLogAndGetRequestTraces(t *testing.T) {
	ctx := setupTestDB(t)

	if err := SaveRequest(ctx, DBRequest{ID: "req-1", Provider: "ollama", Model: "qwen3:8b", PromptTokens: 3, CompletionTokens: 4, Cost: 0.0, LatencyMs: 42, Status: 200, Prompt: "trace me"}); err != nil {
		t.Fatalf("SaveRequest failed: %v", err)
	}
	if err := SaveAuditLog(ctx, DBAuditLog{RequestID: "req-1", OriginalModel: "auto", ChosenProvider: "ollama", ChosenModel: "qwen3:8b", RoutingType: "auto", Complexity: "low", Confidence: 0.9, Reason: "cheap"}); err != nil {
		t.Fatalf("SaveAuditLog failed: %v", err)
	}

	traces, total, err := GetRequestTraces(ctx, TraceFilter{})
	if err != nil {
		t.Fatalf("GetRequestTraces failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	tr := traces[0]
	if tr.RequestID != "req-1" {
		t.Errorf("expected RequestID=req-1, got %s", tr.RequestID)
	}
	// Verifies the LEFT JOIN brought in audit fields.
	if tr.RoutingType != "auto" || tr.Complexity != "low" || tr.ChosenProvider != "ollama" {
		t.Errorf("join did not populate audit fields: %+v", tr)
	}
	if tr.Confidence != 0.9 {
		t.Errorf("expected Confidence=0.9, got %v", tr.Confidence)
	}
	if tr.Prompt != "trace me" {
		t.Errorf("expected Prompt='trace me', got %q", tr.Prompt)
	}

	// Filter by routing type that matches nothing.
	none, noneTotal, err := GetRequestTraces(ctx, TraceFilter{RoutingType: "explicit"})
	if err != nil {
		t.Fatalf("GetRequestTraces(explicit) failed: %v", err)
	}
	if noneTotal != 0 || len(none) != 0 {
		t.Errorf("expected no explicit traces, got %d (total=%d)", len(none), noneTotal)
	}
}

func TestBudgetUpsertGetUpdate(t *testing.T) {
	ctx := setupTestDB(t)

	const key = "sk-test-123"
	if err := UpsertBudgetLimits(ctx, key, "developer", 100.0, 500.0); err != nil {
		t.Fatalf("UpsertBudgetLimits insert failed: %v", err)
	}

	b, err := GetBudget(ctx, key)
	if err != nil {
		t.Fatalf("GetBudget failed: %v", err)
	}
	if b.Role != "developer" || b.DailyLimit != 100.0 || b.MonthlyLimit != 500.0 {
		t.Errorf("unexpected budget after insert: %+v", b)
	}
	if b.DailyUsage != 0 || b.MonthlyUsage != 0 {
		t.Errorf("expected zero usage after insert, got daily=%v monthly=%v", b.DailyUsage, b.MonthlyUsage)
	}

	// Conflict path: upsert again with new limits/role.
	if err := UpsertBudgetLimits(ctx, key, "admin", 1000.0, 5000.0); err != nil {
		t.Fatalf("UpsertBudgetLimits update failed: %v", err)
	}
	b, err = GetBudget(ctx, key)
	if err != nil {
		t.Fatalf("GetBudget after upsert failed: %v", err)
	}
	if b.Role != "admin" || b.DailyLimit != 1000.0 || b.MonthlyLimit != 5000.0 {
		t.Errorf("upsert did not update limits/role: %+v", b)
	}

	// Usage accumulation.
	if err := UpdateBudgetUsage(ctx, key, 12.5); err != nil {
		t.Fatalf("UpdateBudgetUsage failed: %v", err)
	}
	if err := UpdateBudgetUsage(ctx, key, 2.5); err != nil {
		t.Fatalf("UpdateBudgetUsage failed: %v", err)
	}
	b, err = GetBudget(ctx, key)
	if err != nil {
		t.Fatalf("GetBudget after usage failed: %v", err)
	}
	if b.DailyUsage != 15.0 || b.MonthlyUsage != 15.0 {
		t.Errorf("expected usage=15.0, got daily=%v monthly=%v", b.DailyUsage, b.MonthlyUsage)
	}
}

func TestGetBudgetNotFound(t *testing.T) {
	ctx := setupTestDB(t)
	if _, err := GetBudget(ctx, "does-not-exist"); err == nil {
		t.Error("expected error for missing budget, got nil")
	}
}
