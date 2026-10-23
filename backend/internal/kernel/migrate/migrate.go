// Package migrate applies schema migrations: the Platform's own goose SQL files
// (embedded from db/migrations) followed by River's job-queue tables. It is
// invoked by `nexa migrate`, never at server start, so deploys stay explicit.
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// Files is populated by the migrations package via go:embed in the db folder.
// Kept as a variable so tests can swap in a fixture set.
var Files embed.FS

// Up applies all pending migrations. Safe to run repeatedly.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	if err := upGoose(ctx, pool); err != nil {
		return fmt.Errorf("schema migrations: %w", err)
	}
	if err := upRiver(ctx, pool); err != nil {
		return fmt.Errorf("river migrations: %w", err)
	}
	return nil
}

func upGoose(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func(db *sql.DB) { _ = db.Close() }(sqlDB)

	goose.SetBaseFS(Files)
	goose.SetLogger(gooseLogger{})
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, sqlDB, "migrations")
}

func upRiver(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return err
	}
	for _, v := range res.Versions {
		slog.Info("river migration applied", "version", v.Version)
	}
	return nil
}

// gooseLogger routes goose's output through slog so logs stay structured.
type gooseLogger struct{}

func (gooseLogger) Fatalf(format string, v ...any) { slog.Error(fmt.Sprintf(format, v...)) }
func (gooseLogger) Printf(format string, v ...any) { slog.Info(fmt.Sprintf(format, v...)) }
