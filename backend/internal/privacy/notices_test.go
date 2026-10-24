package privacy

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
)

// newTest builds the service with the brand file's notices as its consent
// versions, so tests on the shared brand never write consent versions.
func newTest(d Deps) *Service {
	notices := map[string][]Notice{}
	for _, c := range d.Brand.Legal.Consent {
		notices[c.App] = append(notices[c.App], Notice{App: c.App, Version: c.Version, Title: c.Title, Summary: c.Summary, URL: c.URL, PublishedAt: c.PublishedAt})
	}
	d.Notices = func(context.Context) (map[string][]Notice, error) { return notices, nil }
	return New(d)
}

// throwawayBrand is a brand row only this test uses (removed afterwards).
func throwawayBrand(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	code := fmt.Sprintf("test-%d", rand.IntN(1_000_000))
	if err := pool.QueryRow(ctx, `INSERT INTO brands (code, display_name, country, currency, timezone, default_locale)
		VALUES ($1, 'Test brand', 'ZZ', 'TZS', 'UTC', 'en') RETURNING id`, code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM consent_documents WHERE brand_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM brands WHERE id=$1`, id)
	})
	return id
}

// TestConsentVersionsFromDatabase: the brand file seeds version 1, the starter
// pack links it without a new version, and a publish writes version 2 that
// keeps the heading and asks again.
func TestConsentVersionsFromDatabase(t *testing.T) {
	ctx := context.Background()
	pool, _ := testPool(t)
	brandID := throwawayBrand(t, pool)
	s := New(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1)})
	s.cache = cache.New(noticeScope, s.store, s.store.notices, 0) // check the version on every read
	if err := s.SeedDocuments(ctx); err != nil {
		t.Fatal(err)
	}
	links := []NoticeLink{{Kind: "PRIVACY_NOTICE", Title: map[string]string{"en": "Privacy"}, URL: map[string]string{"en": "/legal/rider/privacy-notice?lang=en"}}}
	write := func(seed bool) int {
		var v int
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) (err error) {
			n := NewNotice{App: AppRider, URL: links[0].URL, Links: links, Documents: map[string]int{"PRIVACY_NOTICE": 1}}
			if seed {
				v, err = s.SeedVersionTx(ctx, tx, n)
			} else {
				v, err = s.NewVersionTx(ctx, tx, n)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		bumpLegal(t, pool)
		return v
	}
	if v := write(true); v != 1 {
		t.Fatalf("the starter pack links version 1, got %d", v)
	}
	doc, ok, err := s.Document(ctx, AppRider, "en")
	if err != nil || !ok || doc.URL != links[0].URL["en"] || len(doc.Links) != 1 || doc.Title != "t" {
		t.Fatalf("linked version 1: %+v %v %v", doc, ok, err)
	}
	user := uuid.NewString()
	if v := write(false); v != 2 {
		t.Fatalf("a publish writes version 2, got %d", v)
	}
	st, err := s.Status(ctx, user, AppRider)
	if err != nil || st.CurrentVersion != 2 || !st.Required {
		t.Fatalf("after a publish everyone is asked again: %+v %v", st, err)
	}
	if doc, _, _ := s.Document(ctx, AppRider, "en"); doc.Title != "t" || doc.Summary != "s" {
		t.Fatalf("a new version keeps the heading and summary: %+v", doc)
	}
}

// bumpLegal moves the "legal" scope so the cache reloads (the legal module does this when it publishes).
func bumpLegal(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE config_versions SET version = version + 1 WHERE scope='legal'`); err != nil {
		t.Fatal(err)
	}
}
