package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	dbfiles "github.com/kalen-limited/hauler-rider/backend/db"
	"github.com/kalen-limited/hauler-rider/backend/internal/areas"
	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/migrate"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/seed"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// runMigrate applies all pending migrations and exits.
func runMigrate() error {
	ctx := context.Background()
	bs, err := load(ctx, true)
	if err != nil {
		return err
	}
	defer bs.pool.Close()
	migrate.Files = dbfiles.Migrations
	if err := migrate.Up(ctx, bs.pool); err != nil {
		return err
	}
	slog.Info("migrations applied")
	return nil
}

// runSeed loads reference data. Idempotent; safe on every deploy.
func runSeed(args []string) error {
	what := "all"
	if len(args) > 0 {
		what = args[0]
	}
	ctx := context.Background()
	bs, err := load(ctx, true)
	if err != nil {
		return err
	}
	defer bs.pool.Close()

	if what == "presets" || what == "all" {
		if err := seed.Presets(ctx, bs.pool, bs.presets); err != nil {
			return err
		}
		slog.Info("theme presets seeded", "count", len(bs.presets.List()))
	}
	if what == "brand" || what == "all" {
		if err := seed.Brand(ctx, bs.pool, bs.brand, bs.presets); err != nil {
			return err
		}
		slog.Info("brand seeded", "brand", bs.brand.Code)
	}
	if what == "catalog" || what == "all" {
		brandID, err := bs.brandID(ctx)
		if err != nil {
			return err
		}
		n, err := catalog.New(bs.pool, brandID).Seed(ctx, filepath.Join(bs.cfg.SeedDir, "catalog"))
		if err != nil {
			return err
		}
		slog.Info("catalogue seeded", "services", n)
	}
	if what == "pricing" || what == "all" {
		brandID, err := bs.brandID(ctx)
		if err != nil {
			return err
		}
		cat := catalog.New(bs.pool, brandID)
		prices := pricing.New(bs.pool, brandID, bs.brand.Country, cat, routing.DefaultEstimate())
		newProfiles(bs, brandID, cat, prices, nil)
		n, err := prices.Seed(ctx, filepath.Join(bs.cfg.SeedDir, "pricing"))
		if err != nil {
			return err
		}
		slog.Info("tariffs seeded", "tariffs", n)
	}
	if what == "areas" || what == "all" {
		brandID, err := bs.brandID(ctx)
		if err != nil {
			return err
		}
		n, err := areas.New(bs.pool, brandID, bs.brand, audit.Record).Seed(ctx, filepath.Join(bs.cfg.SeedDir, "areas"))
		if err != nil {
			return err
		}
		slog.Info("areas seeded", "areas", n)
	}
	if what != "presets" && what != "brand" && what != "catalog" && what != "pricing" && what != "areas" && what != "all" {
		return fmt.Errorf("unknown seed target %q (brand|presets|catalog|pricing|areas|all)", what)
	}
	return nil
}
