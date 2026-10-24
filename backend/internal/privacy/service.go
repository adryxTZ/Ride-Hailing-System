package privacy

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/storage"
)

// Deps are what the service is built from (wired in cmd).
type Deps struct {
	Pool    *pgxpool.Pool
	BrandID string
	Brand   *brand.Brand
	Redis   *redis.Client // nil: the consent gate reads Postgres every time
	Files   storage.Store
	Writer  storage.Writer // nil: exports cannot be written
	Money   Money
	Notify  Notify
	Offline Offline
	// EnqueueExportTx queues the export job in the approving transaction.
	EnqueueExportTx func(ctx context.Context, tx pgx.Tx, requestID string) error
	Location        *time.Location // the brand's home timezone (retention time of day)
	// Notices replaces the consent versions read from the database (tests).
	Notices NoticeSource
}

// noticeScope is the config_versions scope that publishing a legal text bumps.
const noticeScope = "legal"

// Service is the privacy use cases for one brand.
type Service struct {
	d     Deps
	store store
	cache *cache.Versioned[map[string][]Notice] // consent versions per app, oldest first
	rules []Rule
	now   func() time.Time
}

// New builds the service; the retention rules take the brand's day overrides.
// Consent versions live in the database (the legal module publishes them);
// the brand file's only seed an app that has none.
func New(d Deps) *Service {
	if d.Location == nil {
		d.Location = time.UTC
	}
	s := &Service{d: d, store: store{pool: d.Pool, brandID: d.BrandID}, now: time.Now}
	s.cache = cache.New(noticeScope, s.store, s.store.notices, noticeCheckTTL)
	s.rules = s.retentionRules(d.Brand.Privacy.Retention)
	return s
}

// AppOf maps a token's app ("rider", "driver") to a consent app ("" for staff).
func AppOf(tokenApp string) string {
	switch strings.ToLower(tokenApp) {
	case "rider":
		return AppRider
	case "driver":
		return AppDriver
	}
	return ""
}
