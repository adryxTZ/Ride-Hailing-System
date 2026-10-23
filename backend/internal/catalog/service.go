package catalog

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
)

// Catalog is the read API over the cached catalogue for one brand.
type Catalog struct {
	store   store
	brandID string
	cache   *cache.Versioned[Catalogue]
}

// New builds the catalogue reader. The cache re-checks the version at most
// every 5 s, so an admin change is visible within 5 s while steady-state reads
// never touch Postgres.
func New(pool *pgxpool.Pool, brandID string) *Catalog {
	st := store{pool: pool}
	return &Catalog{
		store:   st,
		brandID: brandID,
		cache:   cache.New("catalog", st, func(ctx context.Context) (Catalogue, error) { return st.load(ctx, brandID) }, 5*time.Second),
	}
}

// Catalogue returns the catalogue and its version (used as the HTTP ETag).
func (c *Catalog) Catalogue(ctx context.Context) (Catalogue, int64, error) {
	return c.cache.Get(ctx)
}

// Find returns one service by key, or nil when unknown or disabled.
func (c *Catalog) Find(ctx context.Context, key string) (*Service, error) {
	cat, _, err := c.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	for i := range cat.Services {
		if cat.Services[i].Key == key {
			return &cat.Services[i], nil
		}
	}
	return nil, nil
}

// Seed loads seed/catalog/*.json for the brand (idempotent) and bumps the version.
func (c *Catalog) Seed(ctx context.Context, dir string) (int, error) {
	return c.store.seed(ctx, c.brandID, dir)
}

// ClassExists reports whether an active vehicle class belongs to a service.
func (c *Catalog) ClassExists(ctx context.Context, serviceKey, classKey string) (bool, error) {
	svc, err := c.Find(ctx, serviceKey)
	if err != nil || svc == nil {
		return false, err
	}
	for _, cls := range svc.VehicleClasses {
		if cls.Key == classKey && cls.Status == "ACTIVE" {
			return true, nil
		}
	}
	return false, nil
}

// ServicesForClass lists the active services that offer a vehicle class key
// (a Boda serves both rides and package delivery).
func (c *Catalog) ServicesForClass(ctx context.Context, classKey string) ([]string, error) {
	cat, _, err := c.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, svc := range cat.Services {
		if svc.Status != "ACTIVE" {
			continue
		}
		for _, cls := range svc.VehicleClasses {
			if cls.Key == classKey && cls.Status == "ACTIVE" {
				out = append(out, svc.Key)
				break
			}
		}
	}
	return out, nil
}
