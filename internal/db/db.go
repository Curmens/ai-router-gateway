package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

// DB is the process-wide SQLite handle. database/sql pools connections internally.
var DB *sql.DB

const dbFileName = "airouter.db"

const schemaSQL = `
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    cost REAL DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    status INTEGER DEFAULT 200,
    error_message TEXT,
    prompt TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    original_model TEXT NOT NULL,
    chosen_provider TEXT NOT NULL,
    chosen_model TEXT NOT NULL,
    routing_type TEXT NOT NULL,
    complexity TEXT,
    confidence REAL,
    reason TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS budgets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_key TEXT UNIQUE NOT NULL,
    role TEXT NOT NULL,
    daily_limit REAL NOT NULL,
    daily_usage REAL DEFAULT 0,
    monthly_limit REAL NOT NULL,
    monthly_usage REAL DEFAULT 0,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
`

// resolveDataDir returns the directory that holds the SQLite file. An empty
// configured path defaults to ~/.ai-router; a leading "~" is expanded to the
// user's home directory.
func resolveDataDir(path string) (string, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home dir: %w", err)
		}
		return filepath.Join(home, ".ai-router"), nil
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home dir: %w", err)
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
	}
	return path, nil
}

func InitDB(ctx context.Context, cfg *config.StorageConfig) error {
	dir, err := resolveDataDir(cfg.Path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create data dir %q: %w", dir, err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
		filepath.Join(dir, dbFileName))

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	DB = database
	logger.Log.Info("Successfully opened SQLite database", zap.String("path", filepath.Join(dir, dbFileName)))

	if err := runMigrations(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func runMigrations(ctx context.Context) error {
	for _, stmt := range strings.Split(schemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := DB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration execution failed: %w", err)
		}
	}

	// Add prompt column if it doesn't exist (for databases created before it was added).
	hasPrompt, err := columnExists(ctx, "requests", "prompt")
	if err != nil {
		logger.Log.Error("Failed to inspect requests table", zap.Error(err))
		return err
	}
	if !hasPrompt {
		if _, err := DB.ExecContext(ctx, "ALTER TABLE requests ADD COLUMN prompt TEXT"); err != nil {
			logger.Log.Error("Failed to alter requests table to add prompt column", zap.Error(err))
			return err
		}
	}

	logger.Log.Info("SQLite migrations executed successfully")
	return nil
}

func columnExists(ctx context.Context, table, column string) (bool, error) {
	rows, err := DB.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			dfltValue  sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := DB.ExecContext(ctx, query, req.ID, req.Provider, req.Model, req.PromptTokens, req.CompletionTokens, req.Cost, req.LatencyMs, req.Status, req.ErrorMessage, req.Prompt)
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := DB.ExecContext(ctx, query, log.RequestID, log.OriginalModel, log.ChosenProvider, log.ChosenModel, log.RoutingType, log.Complexity, log.Confidence, log.Reason)
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
		WHERE api_key = ?
	`
	var b DBBudget
	err := DB.QueryRowContext(ctx, query, apiKey).Scan(&b.APIKey, &b.Role, &b.DailyLimit, &b.DailyUsage, &b.MonthlyLimit, &b.MonthlyUsage)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func UpdateBudgetUsage(ctx context.Context, apiKey string, cost float64) error {
	query := `
		UPDATE budgets
		SET daily_usage = daily_usage + ?,
		    monthly_usage = monthly_usage + ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE api_key = ?
	`
	_, err := DB.ExecContext(ctx, query, cost, cost, apiKey)
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

	// Named params let a single value satisfy both the "IS NULL" guard and the
	// equality/range check without listing it twice.
	filterArgs := []any{
		sql.Named("provider", nullableString(f.Provider)),
		sql.Named("model", nullableString(f.Model)),
		sql.Named("from", nullableString(f.From)),
		sql.Named("to", nullableString(f.To)),
	}

	where := `
		(@provider IS NULL OR provider = @provider)
		AND (@model IS NULL OR model = @model)
		AND (@from IS NULL OR created_at >= @from)
		AND (@to IS NULL OR created_at <= @to)
		AND status = 200`

	listArgs := append(append([]any{}, filterArgs...), sql.Named("limit", f.Limit), sql.Named("offset", f.Offset))
	rows, err := DB.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, provider, model, prompt_tokens, completion_tokens, cost, latency_ms, created_at
		FROM requests
		WHERE %s
		ORDER BY created_at DESC
		LIMIT @limit OFFSET @offset`, where), listArgs...)
	if err != nil {
		return nil, UsageSummary{}, fmt.Errorf("failed to query usage logs: %w", err)
	}
	defer rows.Close()

	var entries []UsageLogEntry
	for rows.Next() {
		var e UsageLogEntry
		if err := rows.Scan(&e.ID, &e.Provider, &e.Model, &e.PromptTokens, &e.CompletionTokens, &e.Cost, &e.LatencyMs, &e.CreatedAt); err != nil {
			return nil, UsageSummary{}, fmt.Errorf("failed to scan usage log row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, UsageSummary{}, fmt.Errorf("usage log row iteration error: %w", err)
	}

	var summary UsageSummary
	err = DB.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		       COALESCE(SUM(cost),0), COUNT(*)
		FROM requests
		WHERE %s`, where), filterArgs...).
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

	filterArgs := []any{
		sql.Named("provider", nullableString(f.Provider)),
		sql.Named("routing", nullableString(f.RoutingType)),
		sql.Named("complexity", nullableString(f.Complexity)),
		sql.Named("status", nullableString(f.Status)),
		sql.Named("from", nullableString(f.From)),
		sql.Named("to", nullableString(f.To)),
	}

	where := `
		(@provider IS NULL OR a.chosen_provider = @provider)
		AND (@routing IS NULL OR a.routing_type = @routing)
		AND (@complexity IS NULL OR a.complexity = @complexity)
		AND (@status IS NULL OR CAST(r.status AS TEXT) = @status)
		AND (@from IS NULL OR r.created_at >= @from)
		AND (@to IS NULL OR r.created_at <= @to)`

	listArgs := append(append([]any{}, filterArgs...), sql.Named("limit", f.Limit), sql.Named("offset", f.Offset))
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
		LIMIT @limit OFFSET @offset`, where), listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query request traces: %w", err)
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
			return nil, 0, fmt.Errorf("failed to scan trace row: %w", err)
		}
		traces = append(traces, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("trace row iteration error: %w", err)
	}

	var total int64
	err = DB.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM requests r
		LEFT JOIN audit_logs a ON a.request_id = r.id
		WHERE %s`, where), filterArgs...).Scan(&total)
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
		VALUES (?, ?, ?, ?)
		ON CONFLICT(api_key)
		DO UPDATE SET daily_limit = excluded.daily_limit, monthly_limit = excluded.monthly_limit, role = excluded.role, updated_at = CURRENT_TIMESTAMP
	`
	_, err := DB.ExecContext(ctx, query, apiKey, role, daily, monthly)
	if err != nil {
		logger.Log.Error("Failed to upsert budget limits", zap.Error(err), zap.String("api_key", apiKey))
		return err
	}
	return nil
}
