package main

import (
	"context"
	"fmt"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/rdb"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/config"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/db"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
)

// bootstrap holds what every command needs: config, logger, brand, presets and
// (when configured) the database pool. Commands add their own pieces on top.
type bootstrap struct {
	cfg     *config.Config
	brand   *brand.Brand
	presets *theme.Presets
	pool    *pgxpool.Pool
	redis   *redis.Client // nil when NEXA_REDIS_URL is unset (presence/dispatch disabled)
}

// load reads configuration and the file-based reference data, and opens the
// database when requireDB is set.
func load(ctx context.Context, requireDB bool) (*bootstrap, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel()})))

	b, err := brand.Load(cfg.BrandsDir, cfg.Brand)
	if err != nil {
		return nil, fmt.Errorf("load brand: %w", err)
	}
	presets, err := theme.LoadPresets(cfg.SeedDir)
	if err != nil {
		return nil, fmt.Errorf("load theme presets: %w", err)
	}
	if _, ok := presets.Resolve(b.ThemePreset); !ok {
		return nil, fmt.Errorf("brand %q references unknown theme preset %q", b.Code, b.ThemePreset)
	}
	bs := &bootstrap{cfg: cfg, brand: b, presets: presets}
	if requireDB {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("NEXA_DATABASE_URL is required for this command")
		}
		bs.pool, err = db.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		if cfg.RedisURL != "" {
			bs.redis, err = rdb.Open(ctx, cfg.RedisURL)
			if err != nil {
				return nil, err
			}
		}
	}
	return bs, nil
}

// brandID resolves the database id of the configured brand (seed must have run).
func (bs *bootstrap) brandID(ctx context.Context) (string, error) {
	var id string
	err := bs.pool.QueryRow(ctx, `SELECT id FROM brands WHERE code=$1`, bs.brand.Code).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("brand %q not in database (run `nexa seed brand`): %w", bs.brand.Code, err)
	}
	return id, nil
}
