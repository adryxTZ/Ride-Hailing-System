package privacy

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
)

func testPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	url := os.Getenv("NEXA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEXA_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var brandID string
	if err := pool.QueryRow(context.Background(), `SELECT id FROM brands LIMIT 1`).Scan(&brandID); err != nil {
		t.Skip("no brand")
	}
	return pool, brandID
}

func testBrand(versions ...int) *brand.Brand {
	b := &brand.Brand{Code: "test", DisplayName: "Test", DefaultLocale: "en", Privacy: brand.Privacy{RequestDueDays: 30}}
	for _, v := range versions {
		b.Legal.Consent = append(b.Legal.Consent, brand.ConsentDoc{App: AppRider, Version: v, Title: map[string]string{"en": "t"},
			Summary: map[string]string{"en": "s"}, URL: map[string]string{"en": "u"}, PublishedAt: time.Now()})
	}
	return b
}

// newUser makes a throwaway rider (a unique phone) and removes nothing: consents are append-only.
func newUser(t *testing.T, pool *pgxpool.Pool, brandID string) string {
	t.Helper()
	var id string
	phone := "+2557" + uuid.NewString()[:8]
	if err := pool.QueryRow(context.Background(), `INSERT INTO users (brand_id, phone, full_name) VALUES ($1,$2,'Privacy Test') RETURNING id`, brandID, phone).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

type nopWriter struct{}

func (nopWriter) Put(context.Context, string, io.Reader, int64) error { return nil }
func (nopWriter) Delete(context.Context, string) error                { return nil }

type noMoney struct{ debt decimal.Decimal }

func (m noMoney) DriverDebt(context.Context, string) (decimal.Decimal, string, error) {
	return m.debt, "TZS", nil
}
func (noMoney) DriverOwed(context.Context, string) (decimal.Decimal, string, error) {
	return decimal.Zero, "TZS", nil
}
func (noMoney) RiderDues(context.Context, string) (decimal.Decimal, error) { return decimal.Zero, nil }

func TestConsentGateAndNewVersion(t *testing.T) {
	ctx := context.Background()
	pool, brandID := testPool(t)
	user := newUser(t, pool, brandID)
	s := newTest(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1)})
	if req, _ := s.Required(ctx, user, AppRider); !req {
		t.Fatal("a new person must accept first")
	}
	if _, err := s.Accept(ctx, user, AcceptInput{App: AppRider, Version: 2}); err != ErrConsentStale {
		t.Fatalf("a wrong version must be stale, got %v", err)
	}
	if _, err := s.Accept(ctx, user, AcceptInput{App: AppRider, Version: 1, IP: "127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	if req, _ := s.Required(ctx, user, AppRider); req {
		t.Fatal("after accepting, the gate opens")
	}
	if req, _ := s.Required(ctx, user, AppDriver); req {
		t.Fatal("an app without a notice has no gate")
	}
	v2 := newTest(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1, 2)})
	if req, _ := v2.Required(ctx, user, AppRider); !req {
		t.Fatal("a new version must ask again")
	}
	if _, err := pool.Exec(ctx, `UPDATE user_consents SET version=9 WHERE user_id=$1`, user); err == nil {
		t.Fatal("consents must be append-only")
	}
}

func TestDeleteBlockedThenAnonymised(t *testing.T) {
	ctx := context.Background()
	pool, brandID := testPool(t)
	staff := newUser(t, pool, brandID)
	user := newUser(t, pool, brandID)
	if _, err := pool.Exec(ctx, `INSERT INTO trusted_contacts (user_id, name, phone) VALUES ($1,'Mama','+255700000009')`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO driver_profiles (user_id, brand_id, country_code, national_id, licence_number) VALUES ($1,$2,'TZ','19900101-00000-00000-11','4000000001')`, user, brandID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sessions (family_id, user_id, app, ip, user_agent) VALUES (gen_random_uuid(), $1, 'DRIVER', '10.0.0.1', 'test')`, user); err != nil {
		t.Logf("sessions insert skipped: %v", err)
	}
	blocked := newTest(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1), Money: noMoney{debt: decimal.NewFromInt(500)}})
	r, err := blocked.Create(ctx, user, AppRider, KindDelete)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocked.Create(ctx, user, AppRider, KindDelete); err == nil {
		t.Fatal("one open request per kind")
	}
	if _, err := blocked.Approve(ctx, staff, r.ID, "asked by phone"); err == nil {
		t.Fatal("debt must block deletion")
	}
	s := newTest(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1), Money: noMoney{}})
	done, err := s.Approve(ctx, staff, r.ID, "asked by phone")
	if err != nil || done.Status != StatusDone {
		t.Fatalf("delete: %v", err)
	}
	var name *string
	var phone, status string
	var contacts int
	_ = pool.QueryRow(ctx, `SELECT full_name, phone, status::text, (SELECT count(*) FROM trusted_contacts WHERE user_id=$1) FROM users WHERE id=$1`, user).
		Scan(&name, &phone, &status, &contacts)
	if name != nil || phone != "deleted:"+user || status != "DELETED" || contacts != 0 {
		t.Fatalf("not anonymised: %v %s %s %d", name, phone, status, contacts)
	}
	var nid, licence, ip *string
	_ = pool.QueryRow(ctx, `SELECT national_id, licence_number FROM driver_profiles WHERE user_id=$1`, user).Scan(&nid, &licence)
	_ = pool.QueryRow(ctx, `SELECT ip::text FROM sessions WHERE user_id=$1 LIMIT 1`, user).Scan(&ip)
	if nid != nil || licence != nil || ip != nil {
		t.Fatalf("identity numbers or sign-in data kept: %v %v %v", nid, licence, ip)
	}
}

func TestRetentionRunsOncePerDayAndOnlyOldRows(t *testing.T) {
	ctx := context.Background()
	pool, brandID := testPool(t)
	s := newTest(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1)})
	phone := "+2557" + uuid.NewString()[:8]
	for _, age := range []string{"3 days", "1 hour"} {
		if _, err := pool.Exec(ctx, `INSERT INTO otp_codes (brand_id, phone, app, code_hash, expires_at) VALUES ($1,$2,'RIDER','x', now() - $3::interval)`, brandID, phone, age); err != nil {
			t.Skipf("otp_codes shape differs: %v", err)
		}
	}
	var rule Rule
	for _, r := range s.rules {
		if r.Name == "otp_codes" {
			rule = r
		}
	}
	n, err := rule.Run(ctx, time.Now().Add(-time.Duration(rule.Days)*24*time.Hour))
	if err != nil || n < 1 {
		t.Fatalf("old codes must go: %d %v", n, err)
	}
	var left int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM otp_codes WHERE phone=$1`, phone).Scan(&left)
	if left != 1 {
		t.Fatalf("the recent code must stay, %d left", left)
	}
	day := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	first, _ := s.store.claimRun(ctx, "test_rule_"+uuid.NewString()[:6], day)
	if !first {
		t.Fatal("the first claim of a day wins")
	}
	if again, _ := s.store.claimRun(ctx, "otp_codes", time.Now()); again {
		if again2, _ := s.store.claimRun(ctx, "otp_codes", time.Now()); again2 {
			t.Fatal("a rule runs once a day")
		}
	}
}

// TestEveryRuleRuns runs each rule with a cutoff long ago: its SQL must be valid and touch nothing.
func TestEveryRuleRuns(t *testing.T) {
	ctx := context.Background()
	pool, brandID := testPool(t)
	s := newTest(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1), Writer: nopWriter{}}) // file rules run their queries too
	long := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, r := range s.rules {
		n, err := r.Run(ctx, long)
		if err != nil {
			t.Errorf("%s: %v", r.Name, err)
		}
		if n != 0 {
			t.Errorf("%s touched %d rows older than 2000", r.Name, n)
		}
	}
}

// TestExportSectionsRun builds the export for an existing rider: every section's SQL runs.
func TestExportSectionsRun(t *testing.T) {
	ctx := context.Background()
	pool, brandID := testPool(t)
	var rider string
	if err := pool.QueryRow(ctx, `SELECT rider_id FROM trips ORDER BY requested_at DESC LIMIT 1`).Scan(&rider); err != nil {
		t.Skip("no trips")
	}
	s := newTest(Deps{Pool: pool, BrandID: brandID, Brand: testBrand(1)})
	data, err := s.exportData(ctx, rider)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(exportSections) || string(data["trips"]) == "[]" {
		t.Fatalf("sections %d, trips %s", len(data), data["trips"][:min(len(data["trips"]), 40)])
	}
}
