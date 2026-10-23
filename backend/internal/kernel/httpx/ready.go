package httpx

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/config"
)

// ReadyChecks builds the readiness probe from configuration. Each dependency
// is optional in phase 0; once configured it must answer within two seconds.
func ReadyChecks(ctx context.Context, cfg *config.Config) func(context.Context) error {
	var pool *pgxpool.Pool
	var rdb *redis.Client

	if cfg.DatabaseURL != "" {
		p, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Warn("postgres pool not created", "err", err)
		} else {
			pool = p
		}
	}
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			slog.Warn("redis url invalid", "err", err)
		} else {
			rdb = redis.NewClient(opt)
		}
	}

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if pool != nil {
			if err := pool.Ping(ctx); err != nil {
				return err
			}
		}
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				return err
			}
		}
		return nil
	}
}
