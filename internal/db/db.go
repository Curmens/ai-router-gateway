package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

var Pool *pgxpool.Pool

const schemaSQL = `
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_tokens INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    cost NUMERIC(10, 6) DEFAULT 0.000000,
    latency_ms INT DEFAULT 0,
    status INT DEFAULT 200,
    error_message TEXT,
    prompt TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    original_model TEXT NOT NULL,
    chosen_provider TEXT NOT NULL,
    chosen_model TEXT NOT NULL,
    routing_type TEXT NOT NULL,
    complexity TEXT,
    confidence NUMERIC(4, 2),
    reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS budgets (
    id SERIAL PRIMARY KEY,
    api_key TEXT UNIQUE NOT NULL,
    role TEXT NOT NULL,
    daily_limit NUMERIC(10, 2) NOT NULL,
    daily_usage NUMERIC(10, 6) DEFAULT 0.000000,
    monthly_limit NUMERIC(10, 2) NOT NULL,
    monthly_usage NUMERIC(10, 6) DEFAULT 0.000000,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

func InitDB(ctx context.Context, cfg *config.DatabaseConfig) error {
	pgxConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return fmt.Errorf("failed to parse postgres config: %w", err)
	}

	pgxConfig.MaxConns = int32(cfg.MaxConnections)
	pgxConfig.MinConns = int32(cfg.MinConnections)

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	Pool = pool
	logger.Log.Info("Successfully connected to PostgreSQL")

	if err := runMigrations(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func runMigrations(ctx context.Context) error {
	_, err := Pool.Exec(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("migration execution failed: %w", err)
	}
	// Add prompt column if it doesn't exist (for existing databases)
	_, err = Pool.Exec(ctx, "ALTER TABLE requests ADD COLUMN IF NOT EXISTS prompt TEXT;")
	if err != nil {
		logger.Log.Error("Failed to alter requests table to add prompt column", zap.Error(err))
		return err
	}
	logger.Log.Info("PostgreSQL migrations executed successfully")
	return nil
}

type DBRequest struct {
	ID               string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	Cost             float64
	LatencyMs        int
	Status           int
	ErrorMessage     string
	Prompt           string
}

func SaveRequest(ctx context.Context, req DBRequest) error {
	query := `
		INSERT INTO requests (id, provider, model, prompt_tokens, completion_tokens, cost, latency_ms, status, error_message, prompt)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := Pool.Exec(ctx, query, req.ID, req.Provider, req.Model, req.PromptTokens, req.CompletionTokens, req.Cost, req.LatencyMs, req.Status, req.ErrorMessage, req.Prompt)
	if err != nil {
		logger.Log.Error("Failed to save request to DB", zap.Error(err), zap.String("request_id", req.ID))
		return err
	}
	return nil
}

type DBAuditLog struct {
	RequestID      string
	OriginalModel  string
	ChosenProvider string
	ChosenModel    string
	RoutingType    string
	Complexity     string
	Confidence     float64
	Reason         string
}

func SaveAuditLog(ctx context.Context, log DBAuditLog) error {
	query := `
		INSERT INTO audit_logs (request_id, original_model, chosen_provider, chosen_model, routing_type, complexity, confidence, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := Pool.Exec(ctx, query, log.RequestID, log.OriginalModel, log.ChosenProvider, log.ChosenModel, log.RoutingType, log.Complexity, log.Confidence, log.Reason)
	if err != nil {
		logger.Log.Error("Failed to save audit log to DB", zap.Error(err), zap.String("request_id", log.RequestID))
		return err
	}
	return nil
}

type DBBudget struct {
	APIKey       string
	Role         string
	DailyLimit   float64
	DailyUsage   float64
	MonthlyLimit float64
	MonthlyUsage float64
}

func GetBudget(ctx context.Context, apiKey string) (*DBBudget, error) {
	query := `
		SELECT api_key, role, daily_limit, daily_usage, monthly_limit, monthly_usage
		FROM budgets
		WHERE api_key = $1
	`
	var b DBBudget
	err := Pool.QueryRow(ctx, query, apiKey).Scan(&b.APIKey, &b.Role, &b.DailyLimit, &b.DailyUsage, &b.MonthlyLimit, &b.MonthlyUsage)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func UpdateBudgetUsage(ctx context.Context, apiKey string, cost float64) error {
	query := `
		UPDATE budgets
		SET daily_usage = daily_usage + $2,
		    monthly_usage = monthly_usage + $2,
		    updated_at = NOW()
		WHERE api_key = $1
	`
	_, err := Pool.Exec(ctx, query, apiKey, cost)
	if err != nil {
		logger.Log.Error("Failed to update budget usage in DB", zap.Error(err), zap.String("api_key", apiKey))
		return err
	}
	return nil
}

type UsageLogEntry struct {
	ID               string  `json:"id"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	LatencyMs        int     `json:"latency_ms"`
	CreatedAt        string  `json:"created_at"`
}

type UsageLogFilter struct {
	Provider string
	Model    string
	From     string
	To       string
	Limit    int
	Offset   int
}

type UsageSummary struct {
	TotalPromptTokens     int64
	TotalCompletionTokens int64
	TotalCost             float64
	RequestCount          int64
}

func GetUsageLogs(ctx context.Context, f UsageLogFilter) ([]UsageLogEntry, UsageSummary, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}

	args := []any{
		nullableString(f.Provider),
		nullableString(f.Model),
		nullableString(f.From),
		nullableString(f.To),
	}

	where := `
		($1::text IS NULL OR provider = $1)
		AND ($2::text IS NULL OR model = $2)
		AND ($3::timestamp IS NULL OR created_at >= $3::timestamp)
		AND ($4::timestamp IS NULL OR created_at <= $4::timestamp)
		AND status = 200`

	rows, err := Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, provider, model, prompt_tokens, completion_tokens, cost, latency_ms, created_at
		FROM requests
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6`, where),
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, UsageSummary{}, fmt.Errorf("failed to query usage logs: %w", err)
	}
	defer rows.Close()

	var entries []UsageLogEntry
	for rows.Next() {
		var e UsageLogEntry
		var createdAt any
		if err := rows.Scan(&e.ID, &e.Provider, &e.Model, &e.PromptTokens, &e.CompletionTokens, &e.Cost, &e.LatencyMs, &createdAt); err != nil {
			return nil, UsageSummary{}, fmt.Errorf("failed to scan usage log row: %w", err)
		}
		if t, ok := createdAt.(interface{ Format(string) string }); ok {
			e.CreatedAt = t.Format("2006-01-02T15:04:05Z")
		} else {
			e.CreatedAt = fmt.Sprintf("%v", createdAt)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, UsageSummary{}, fmt.Errorf("usage log row iteration error: %w", err)
	}

	var summary UsageSummary
	err = Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		       COALESCE(SUM(cost),0), COUNT(*)
		FROM requests
		WHERE %s`, where), args...).
		Scan(&summary.TotalPromptTokens, &summary.TotalCompletionTokens, &summary.TotalCost, &summary.RequestCount)
	if err != nil {
		return nil, UsageSummary{}, fmt.Errorf("failed to query usage summary: %w", err)
	}

	return entries, summary, nil
}

type RequestTrace struct {
	RequestID        string  `json:"id"`
	OriginalModel    string  `json:"original_model"`
	ChosenProvider   string  `json:"provider"`
	ChosenModel      string  `json:"model"`
	RoutingType      string  `json:"routing_type"`
	Complexity       string  `json:"complexity"`
	Confidence       float64 `json:"confidence"`
	Reason           string  `json:"reason"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	LatencyMs        int     `json:"latency_ms"`
	Status           int     `json:"status"`
	ErrorMessage     string  `json:"error_message"`
	CreatedAt        string  `json:"created_at"`
	Prompt           string  `json:"prompt"`
}

type TraceFilter struct {
	Provider    string
	RoutingType string
	Complexity  string
	Status      string
	From        string
	To          string
	Limit       int
	Offset      int
}

func GetRequestTraces(ctx context.Context, f TraceFilter) ([]RequestTrace, int64, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}

	args := []any{
		nullableString(f.Provider),
		nullableString(f.RoutingType),
		nullableString(f.Complexity),
		nullableString(f.Status),
		nullableString(f.From),
		nullableString(f.To),
	}

	where := `
		($1::text IS NULL OR a.chosen_provider = $1)
		AND ($2::text IS NULL OR a.routing_type = $2)
		AND ($3::text IS NULL OR a.complexity = $3)
		AND ($4::text IS NULL OR r.status::text = $4)
		AND ($5::timestamp IS NULL OR r.created_at >= $5::timestamp)
		AND ($6::timestamp IS NULL OR r.created_at <= $6::timestamp)`

	rows, err := Pool.Query(ctx, fmt.Sprintf(`
		SELECT
			r.id,
			COALESCE(a.original_model, r.model),
			COALESCE(a.chosen_provider, r.provider),
			COALESCE(a.chosen_model, r.model),
			COALESCE(a.routing_type, 'explicit'),
			COALESCE(a.complexity, ''),
			COALESCE(a.confidence, 0),
			COALESCE(a.reason, ''),
			r.prompt_tokens, r.completion_tokens,
			r.cost, r.latency_ms, r.status, COALESCE(r.error_message, ''),
			r.created_at, COALESCE(r.prompt, '')
		FROM requests r
		LEFT JOIN audit_logs a ON a.request_id = r.id
		WHERE %s
		ORDER BY r.created_at DESC
		LIMIT $7 OFFSET $8`, where),
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query request traces: %w", err)
	}
	defer rows.Close()

	var traces []RequestTrace
	for rows.Next() {
		var t RequestTrace
		var createdAt any
		if err := rows.Scan(
			&t.RequestID, &t.OriginalModel, &t.ChosenProvider, &t.ChosenModel,
			&t.RoutingType, &t.Complexity, &t.Confidence,
			&t.Reason, &t.PromptTokens, &t.CompletionTokens,
			&t.Cost, &t.LatencyMs, &t.Status, &t.ErrorMessage,
			&createdAt, &t.Prompt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan trace row: %w", err)
		}
		if tt, ok := createdAt.(interface{ Format(string) string }); ok {
			t.CreatedAt = tt.Format("2006-01-02T15:04:05Z")
		} else {
			t.CreatedAt = fmt.Sprintf("%v", createdAt)
		}
		traces = append(traces, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("trace row iteration error: %w", err)
	}

	var total int64
	err = Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM requests r
		LEFT JOIN audit_logs a ON a.request_id = r.id
		WHERE %s`, where), args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count traces: %w", err)
	}

	return traces, total, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func UpsertBudgetLimits(ctx context.Context, apiKey string, role string, daily float64, monthly float64) error {
	query := `
		INSERT INTO budgets (api_key, role, daily_limit, monthly_limit)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (api_key) 
		DO UPDATE SET daily_limit = $3, monthly_limit = $4, role = $2, updated_at = NOW()
	`
	_, err := Pool.Exec(ctx, query, apiKey, role, daily, monthly)
	if err != nil {
		logger.Log.Error("Failed to upsert budget limits", zap.Error(err), zap.String("api_key", apiKey))
		return err
	}
	return nil
}
