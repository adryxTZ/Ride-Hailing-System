package legal

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
)

// cacheCheckTTL bounds how long another instance serves an old text after a publish.
const cacheCheckTTL = 5 * time.Second

// cacheScope is the config_versions scope publishing bumps (shared with consent).
const cacheScope = "legal"

// adminLocale is the dashboard's language (kind names on the Legal page).
const adminLocale = "en"

// Deps are what the service is built from (wired in cmd).
type Deps struct {
	Pool     *pgxpool.Pool
	BrandID  string
	Brand    *brand.Brand
	Identity IdentitySource   // nil: the brand file's identity
	Consent  ConsentPublisher // nil: publishing asks nobody again (tests, tools)
}

// snapshot is what the public pages read: the live versions and the details.
type snapshot struct {
	live    map[string]*Document // docKey(app, kind) -> published version
	details map[string]string
}

// Service is the legal-texts use cases for one brand.
type Service struct {
	d     Deps
	store store
	cache *cache.Versioned[snapshot]
	now   func() time.Time
}

// New builds the service.
func New(d Deps) *Service {
	s := &Service{d: d, store: store{pool: d.Pool, brandID: d.BrandID}, now: time.Now}
	s.cache = cache.New(cacheScope, s.store, s.loadSnapshot, cacheCheckTTL)
	return s
}

func docKey(app, kind string) string { return app + "/" + kind }

func (s *Service) loadSnapshot(ctx context.Context) (snapshot, error) {
	docs, err := s.store.live(ctx, s.store.pool)
	if err != nil {
		return snapshot{}, err
	}
	det, err := s.store.details(ctx, s.store.pool)
	if err != nil {
		return snapshot{}, err
	}
	snap := snapshot{live: map[string]*Document{}, details: det.Fields}
	for i := range docs {
		snap.live[docKey(docs[i].App, docs[i].Kind)] = &docs[i]
	}
	return snap, nil
}

// Overview lists every document the apps have, with its live version and draft.
func (s *Service) Overview(ctx context.Context) ([]Overview, error) {
	docs, err := s.store.current(ctx)
	if err != nil {
		return nil, err
	}
	out := []Overview{}
	for _, app := range Apps {
		for _, k := range Kinds {
			if Offered(app, k.Kind) {
				out = append(out, overviewOf(app, k, docs, adminLocale))
			}
		}
	}
	return out, nil
}

// overviewOf picks one document's live version and draft from the current rows.
func overviewOf(app string, k KindInfo, docs []Document, locale string) Overview {
	o := Overview{App: app, Kind: k.Kind, Slug: k.Slug, Consent: k.Consent, Title: pick(k.Title, locale)}
	for i := range docs {
		d := &docs[i]
		if d.App != app || d.Kind != k.Kind {
			continue
		}
		ref := &VersionRef{ID: d.ID, Version: d.Version, Source: d.Source, UpdatedAt: d.UpdatedAt, PublishedAt: d.PublishedAt}
		if d.Status == StatusPublished {
			o.Live = ref
		} else {
			o.Draft = ref
		}
	}
	return o
}

// Versions lists one document's versions, newest first (bodies included, for the editor and diff).
func (s *Service) Versions(ctx context.Context, app, kind string) ([]Document, error) {
	if !Offered(app, kind) {
		return nil, ErrNotFound
	}
	return s.store.versions(ctx, app, kind)
}

// pick is a text in a language, else in the fallback language, else English.
func pick(t Text, lang string, fallback ...string) string {
	if v := t[lang]; v != "" {
		return v
	}
	for _, f := range append(fallback, "en") {
		if v := t[f]; v != "" {
			return v
		}
	}
	return ""
}
