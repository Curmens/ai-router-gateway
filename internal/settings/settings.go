// Package settings persists user-configurable settings in the SQLite filedb.
// YAML seeds the DB on first boot; afterwards the DB is the source of truth.
package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/db"
)

const ProvidersKey = "providers"

// Load overlays persisted providers onto cfg, seeding from cfg on first boot.
func Load(ctx context.Context, cfg *config.Config) error {
	val, found, err := db.GetSetting(ctx, ProvidersKey)
	if err != nil {
		return fmt.Errorf("failed to read persisted providers: %w", err)
	}
	if !found {
		return SaveProviders(ctx, cfg)
	}

	var p config.ProvidersConfig
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return fmt.Errorf("failed to unmarshal persisted providers: %w", err)
	}
	cfg.Providers = p
	return nil
}

// SaveProviders persists cfg.Providers as JSON in the filedb.
func SaveProviders(ctx context.Context, cfg *config.Config) error {
	b, err := json.Marshal(cfg.Providers)
	if err != nil {
		return fmt.Errorf("failed to marshal providers: %w", err)
	}
	return db.SetSetting(ctx, ProvidersKey, string(b))
}
