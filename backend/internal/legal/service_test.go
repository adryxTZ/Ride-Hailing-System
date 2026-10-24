package legal

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
)

// fakeConsent records the consent versions publishing asks for.
type fakeConsent struct {
	published []ConsentVersion
	seeded    []ConsentVersion
}

func (f *fakeConsent) NewVersionTx(_ context.Context, _ pgx.Tx, v ConsentVersion) (int, error) {
	f.published = append(f.published, v)
	return len(f.published) + 1, nil
}

func (f *fakeConsent) SeedVersionTx(_ context.Context, _ pgx.Tx, v ConsentVersion) (int, error) {
	f.seeded = append(f.seeded, v)
	return 1, nil
}

func testBrand() *brand.Brand {
	return &brand.Brand{Code: "test", DisplayName: "Ride Co", LegalEntity: "Ride Co Ltd", DefaultLocale: "en", Locales: []string{"en", "sw"},
		Country: "ZZ", Timezone: "UTC", SupportEmail: "help@ride.example",
		Countries: []brand.Country{{Code: "ZZ", Name: map[string]string{"en": "Testland", "sw": "Nchi ya Majaribio"}, Regulator: "latra"}}}
}

// newTestService runs against a throwaway brand row (removed afterwards). Needs NEXA_TEST_DATABASE_URL.
func newTestService(t *testing.T) (*Service, *fakeConsent, string) {
	t.Helper()
	url := os.Getenv("NEXA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEXA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var brandID, staff string
	if err := pool.QueryRow(ctx, `INSERT INTO brands (code, display_name, country, currency, timezone, default_locale)
		VALUES ($1, 'Ride Co', 'ZZ', 'TZS', 'UTC', 'en') RETURNING id`, fmt.Sprintf("test-%d", rand.IntN(1_000_000))).Scan(&brandID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeBrand(t, pool, brandID) })
	if err := pool.QueryRow(ctx, `SELECT id FROM users ORDER BY created_at LIMIT 1`).Scan(&staff); err != nil {
		t.Skip("no user to act as staff")
	}
	consent := &fakeConsent{}
	s := New(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(), Consent: consent})
	s.cache = cache.New(cacheScope, s.store, s.loadSnapshot, 0) // check the version on every read
	return s, consent, staff
}

// removeBrand deletes the test brand's rows. Published texts and the audit log
// are protected by triggers, switched off inside this one transaction only
// (DDL is transactional: no other session ever sees them off).
func removeBrand(t *testing.T, pool *pgxpool.Pool, brandID string) {
	ctx := context.Background()
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for _, sql := range []string{
			`ALTER TABLE legal_documents DISABLE TRIGGER legal_documents_frozen`,
			`ALTER TABLE audit_log DISABLE TRIGGER audit_log_no_change`,
			`DELETE FROM legal_documents WHERE brand_id=$1`,
			`DELETE FROM legal_details WHERE brand_id=$1`,
			`DELETE FROM audit_log WHERE brand_id=$1`,
			`DELETE FROM brands WHERE id=$1`,
			`ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_change`,
			`ALTER TABLE legal_documents ENABLE TRIGGER legal_documents_frozen`,
		} {
			args := []any{brandID}
			if strings.HasPrefix(sql, "ALTER") {
				args = nil
			}
			if _, err := tx.Exec(ctx, sql, args...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Logf("test brand %s not removed: %v", brandID, err)
	}
}

func TestStarterThenEditAndPublish(t *testing.T) {
	ctx := context.Background()
	s, consent, staff := newTestService(t)
	pack, err := LoadStarter(starterDir)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.SeedStarter(ctx, pack); err != nil || n != len(pack.Documents) {
		t.Fatalf("starter pack: %d %v", n, err)
	}
	if n, _ := s.SeedStarter(ctx, pack); n != 0 {
		t.Fatal("the starter pack is seeded once")
	}
	if len(consent.seeded) != 2 || consent.seeded[1].Documents[KindDriverAgreement] != 1 {
		t.Fatalf("both apps' consent links the starter texts: %+v", consent.seeded)
	}
	if _, err := s.SaveDraft(ctx, staff, AppRider, KindTerms, DraftInput{Body: Text{"en": "Hi {{dpo_mial}}"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an unknown placeholder is refused, got %v", err)
	}
	draft := DraftInput{Title: Text{"en": "Terms", "sw": "Masharti"}, Body: Text{"en": "## New\n\nWrite to {{dpo_email}}.", "sw": "## Mpya"}}
	d, err := s.SaveDraft(ctx, staff, AppRider, KindTerms, draft)
	if err != nil || d.Version != 2 || d.Status != StatusDraft {
		t.Fatalf("draft: %+v %v", d, err)
	}
	if _, err := s.Publish(ctx, staff, AppRider, KindTerms, " "); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("publish needs a reason, got %v", err)
	}
	live, err := s.Publish(ctx, staff, AppRider, KindTerms, "Lawyer reviewed")
	if err != nil || live.Status != StatusPublished {
		t.Fatalf("publish: %+v %v", live, err)
	}
	if len(consent.published) != 1 || consent.published[0].Documents[KindTerms] != 2 || len(consent.published[0].Links) != 2 {
		t.Fatalf("publishing terms writes a consent version covering them: %+v", consent.published)
	}
	checkHistoryAndPage(t, s, staff)
}

func checkHistoryAndPage(t *testing.T, s *Service, staff string) {
	t.Helper()
	ctx := context.Background()
	versions, _ := s.Versions(ctx, AppRider, KindTerms)
	if len(versions) != 2 || versions[1].Status != StatusArchived {
		t.Fatalf("v1 is archived: %+v", versions)
	}
	if _, err := s.store.pool.Exec(ctx, `UPDATE legal_documents SET body='{}' WHERE id=$1`, versions[0].ID); err == nil {
		t.Fatal("a published text must never change")
	}
	if _, err := s.SetDetails(ctx, staff, DetailsInput{Fields: map[string]string{"dpo_email": "not an email"}, Reason: "x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a bad e-mail is refused, got %v", err)
	}
	if _, err := s.SetDetails(ctx, staff, DetailsInput{Fields: map[string]string{"dpo_email": "dpo@ride.example"}, Reason: "DPO named"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.Page(ctx, AppRider, KindTerms, "en")
	if err != nil || !strings.Contains(p.HTML, `href="mailto:dpo@ride.example"`) || p.Version != 2 || p.Brand != "Ride Co" {
		t.Fatalf("page: %+v %v", p, err)
	}
	if sw, _ := s.Page(ctx, AppRider, KindTerms, "sw"); sw.Lang != "sw" || !strings.Contains(sw.HTML, "Mpya") {
		t.Fatalf("Swahili page: %+v", sw)
	}
}
