package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

var DB *sql.DB

func InitDB(ctx context.Context, cfg *config.DatabaseConfig) error {
	path := cfg.Path
	if path == "" {
		path = "router.db"
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("failed to open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping sqlite: %w", err)
	}

	DB = db
	logger.Log.Info("SQLite database initialized", zap.String("path", path))
	return runMigrations(ctx)
}

func runMigrations(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS requests (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			cost REAL DEFAULT 0.0,
			latency_ms INTEGER DEFAULT 0,
			status INTEGER DEFAULT 200,
			error_message TEXT,
			prompt TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			original_model TEXT NOT NULL,
			chosen_provider TEXT NOT NULL,
			chosen_model TEXT NOT NULL,
			routing_type TEXT NOT NULL,
			complexity TEXT,
			confidence REAL,
			reason TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS budgets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			api_key TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL,
			daily_limit REAL NOT NULL,
			daily_usage REAL DEFAULT 0.0,
			monthly_limit REAL NOT NULL,
			monthly_usage REAL DEFAULT 0.0,
			updated_at TEXT DEFAULT (datetime('now'))
		)`,
	}

	for _, stmt := range stmts {
		if _, err := DB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	logger.Log.Info("SQLite migrations executed successfully")
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
	_, err := DB.ExecContext(ctx,
		`INSERT INTO requests (id, provider, model, prompt_tokens, completion_tokens, cost, latency_ms, status, error_message, prompt)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Provider, req.Model, req.PromptTokens, req.CompletionTokens,
		req.Cost, req.LatencyMs, req.Status, req.ErrorMessage, req.Prompt,
	)
	if err != nil {
		logger.Log.Error("Failed to save request to DB", zap.Error(err), zap.String("request_id", req.ID))
	}
	return err
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
	_, err := DB.ExecContext(ctx,
		`INSERT INTO audit_logs (request_id, original_model, chosen_provider, chosen_model, routing_type, complexity, confidence, reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.RequestID, log.OriginalModel, log.ChosenProvider, log.ChosenModel,
		log.RoutingType, log.Complexity, log.Confidence, log.Reason,
	)
	if err != nil {
		logger.Log.Error("Failed to save audit log to DB", zap.Error(err), zap.String("request_id", log.RequestID))
	}
	return err
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
	var b DBBudget
	err := DB.QueryRowContext(ctx,
		`SELECT api_key, role, daily_limit, daily_usage, monthly_limit, monthly_usage FROM budgets WHERE api_key = ?`,
		apiKey,
	).Scan(&b.APIKey, &b.Role, &b.DailyLimit, &b.DailyUsage, &b.MonthlyLimit, &b.MonthlyUsage)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func UpdateBudgetUsage(ctx context.Context, apiKey string, cost float64) error {
	_, err := DB.ExecContext(ctx,
		`UPDATE budgets SET daily_usage = daily_usage + ?, monthly_usage = monthly_usage + ?, updated_at = datetime('now') WHERE api_key = ?`,
		cost, cost, apiKey,
	)
	if err != nil {
		logger.Log.Error("Failed to update budget usage in DB", zap.Error(err), zap.String("api_key", apiKey))
	}
	return err
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

	conds := []string{"status = 200"}
	args := []any{}

	if f.Provider != "" {
		conds = append(conds, "provider = ?")
		args = append(args, f.Provider)
	}
	if f.Model != "" {
		conds = append(conds, "model = ?")
		args = append(args, f.Model)
	}
	if f.From != "" {
		conds = append(conds, "created_at >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		conds = append(conds, "created_at <= ?")
		args = append(args, f.To)
	}

	where := strings.Join(conds, " AND ")

	rows, err := DB.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, provider, model, prompt_tokens, completion_tokens, cost, latency_ms, created_at
		FROM requests WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, where),
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, UsageSummary{}, fmt.Errorf("failed to query usage logs: %w", err)
	}
	defer rows.Close()

	var entries []UsageLogEntry
	for rows.Next() {
		var e UsageLogEntry
		if err := rows.Scan(&e.ID, &e.Provider, &e.Model, &e.PromptTokens, &e.CompletionTokens, &e.Cost, &e.LatencyMs, &e.CreatedAt); err != nil {
			return nil, UsageSummary{}, fmt.Errorf("failed to scan usage log: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, UsageSummary{}, fmt.Errorf("row iteration error: %w", err)
	}

	var summary UsageSummary
	err = DB.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(cost),0), COUNT(*)
		FROM requests WHERE %s`, where), args...).
		Scan(&summary.TotalPromptTokens, &summary.TotalCompletionTokens, &summary.TotalCost, &summary.RequestCount)
	if err != nil {
		return nil, UsageSummary{}, fmt.Errorf("failed to query summary: %w", err)
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

	var conds []string
	var args []any

	if f.Provider != "" {
		conds = append(conds, "a.chosen_provider = ?")
		args = append(args, f.Provider)
	}
	if f.RoutingType != "" {
		conds = append(conds, "a.routing_type = ?")
		args = append(args, f.RoutingType)
	}
	if f.Complexity != "" {
		conds = append(conds, "a.complexity = ?")
		args = append(args, f.Complexity)
	}
	if f.Status != "" {
		conds = append(conds, "CAST(r.status AS TEXT) = ?")
		args = append(args, f.Status)
	}
	if f.From != "" {
		conds = append(conds, "r.created_at >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		conds = append(conds, "r.created_at <= ?")
		args = append(args, f.To)
	}

	where := "1=1"
	if len(conds) > 0 {
		where = strings.Join(conds, " AND ")
	}

	rows, err := DB.QueryContext(ctx, fmt.Sprintf(`
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
		LIMIT ? OFFSET ?`, where),
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query traces: %w", err)
	}
	defer rows.Close()

	var traces []RequestTrace
	for rows.Next() {
		var t RequestTrace
		if err := rows.Scan(
			&t.RequestID, &t.OriginalModel, &t.ChosenProvider, &t.ChosenModel,
			&t.RoutingType, &t.Complexity, &t.Confidence,
			&t.Reason, &t.PromptTokens, &t.CompletionTokens,
			&t.Cost, &t.LatencyMs, &t.Status, &t.ErrorMessage,
			&t.CreatedAt, &t.Prompt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan trace: %w", err)
		}
		traces = append(traces, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("row iteration error: %w", err)
	}

	var total int64
	err = DB.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM requests r
		LEFT JOIN audit_logs a ON a.request_id = r.id
		WHERE %s`, where), args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count traces: %w", err)
	}

	return traces, total, nil
}

func UpsertBudgetLimits(ctx context.Context, apiKey, role string, daily, monthly float64) error {
	_, err := DB.ExecContext(ctx, `
		INSERT INTO budgets (api_key, role, daily_limit, monthly_limit)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(api_key) DO UPDATE SET
			daily_limit = excluded.daily_limit,
			monthly_limit = excluded.monthly_limit,
			role = excluded.role,
			updated_at = datetime('now')`,
		apiKey, role, daily, monthly,
	)
	if err != nil {
		logger.Log.Error("Failed to upsert budget limits", zap.Error(err), zap.String("api_key", apiKey))
	}
	return err
}
