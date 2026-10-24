package privacy

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
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
}

// Service is the privacy use cases for one brand.
type Service struct {
	d     Deps
	store store
	docs  map[string][]brand.ConsentDoc // per app, newest version last
	rules []Rule
	now   func() time.Time
}

// New builds the service; the retention rules take the brand's day overrides.
func New(d Deps) *Service {
	if d.Location == nil {
		d.Location = time.UTC
	}
	s := &Service{d: d, store: store{pool: d.Pool, brandID: d.BrandID}, docs: map[string][]brand.ConsentDoc{}, now: time.Now}
	for _, doc := range d.Brand.Legal.Consent {
		s.docs[doc.App] = append(s.docs[doc.App], doc)
	}
	for app := range s.docs {
		sortByVersion(s.docs[app])
	}
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

func sortByVersion(docs []brand.ConsentDoc) {
	for i := 1; i < len(docs); i++ {
		for j := i; j > 0 && docs[j].Version < docs[j-1].Version; j-- {
			docs[j], docs[j-1] = docs[j-1], docs[j]
		}
	}
}
