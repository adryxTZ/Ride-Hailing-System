package payments

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment/fake"
)

var testLimits = holds.Limits{Soft: decimal.NewFromInt(1000), Hard: decimal.NewFromInt(5000), GraceDays: 7, MinPayin: decimal.NewFromInt(1000)}

type fixture struct {
	svc    *Service
	holds  *holds.Service
	ledger *money.Ledger
	pool   *pgxpool.Pool
	driver string
	gw     *fake.Gateway
	now    time.Time
}

// newFixture needs the compose database (NEXA_TEST_DATABASE_URL), migrated and seeded.
// The driver starts with 6,000 TZS of cash-trip debt and is therefore on hold.
func newFixture(t *testing.T) *fixture {
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
	ctx := context.Background()
	var brandID, driver string
	if err := pool.QueryRow(ctx, `SELECT id FROM brands ORDER BY created_at LIMIT 1`).Scan(&brandID); err != nil {
		t.Skip("no seeded brand")
	}
	phone := "+2557" + time.Now().Format("0405.000000")[0:9]
	if err := pool.QueryRow(ctx, `INSERT INTO users (brand_id, phone, full_name) VALUES ($1,$2,'Pay Test') RETURNING id`, brandID, phone).Scan(&driver); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO driver_profiles (user_id, brand_id, country_code) SELECT $1, $2, country FROM brands WHERE id=$2`, driver, brandID)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM audit_log WHERE target=$1 OR target LIKE 'payin:%'`, "driver:"+driver)
		_, _ = pool.Exec(ctx, `DELETE FROM gateway_transactions WHERE payer_id=$1`, driver)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_holds WHERE driver_id=$1`, driver)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_debt_clock WHERE driver_id=$1`, driver)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_profiles WHERE user_id=$1`, driver)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, driver)
		// Ledger rows are append-only by design; test accounts stay but never affect other drivers.
	})
	f := &fixture{pool: pool, driver: driver, now: time.Now()}
	f.ledger = money.NewLedger(pool, brandID)
	f.gw = &fake.Gateway{Delay: 0, Now: func() time.Time { return f.now }}
	limits := func(context.Context) (holds.Limits, error) { return testLimits, nil }
	f.holds = holds.New(pool, brandID, f.ledger, limits, nil, nil)
	f.svc = New(Deps{Pool: pool, BrandID: brandID, Country: "TZ", Gateways: payment.NewGateways(map[string]payment.Gateway{"TZS": f.gw}), Debts: f.ledger, Ledger: f.ledger, Holds: f.holds, Limits: limits})
	f.svc.now = func() time.Time { return f.now }
	// Cash trip: the driver keeps the fare and owes the platform 6,000.
	if _, err := f.ledger.Post(ctx, "trip.completed", "test-"+driver, "test debt", "TZS", []money.Leg{
		{Kind: money.DriverWallet, OwnerID: driver, Amount: decimal.NewFromInt(6000)}, {Kind: money.PlatformRevenue, Amount: decimal.NewFromInt(-6000)}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.holds.Evaluate(ctx, driver); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) debt(t *testing.T) decimal.Decimal {
	t.Helper()
	d, _, err := f.ledger.DriverDebt(context.Background(), f.driver)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCreateValidatesAndIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, _, err := f.svc.Create(ctx, f.driver, "short", decimal.NewFromInt(1000), "0712000000"); !errors.Is(err, ErrKeyRequired) {
		t.Fatalf("short key: %v", err)
	}
	if _, _, err := f.svc.Create(ctx, f.driver, "key-below-min", decimal.NewFromInt(999), "0712000000"); !errors.Is(err, ErrBelowMinimum) {
		t.Fatalf("below minimum: %v", err)
	}
	if _, _, err := f.svc.Create(ctx, f.driver, "key-above-debt", decimal.NewFromInt(6001), "0712000000"); !errors.Is(err, ErrAboveDebt) {
		t.Fatalf("above debt: %v", err)
	}
	p, replayed, err := f.svc.Create(ctx, f.driver, "key-first-payin", decimal.NewFromInt(1000), "0712000009")
	if err != nil || replayed || p.Status != StatusPending || p.Phone != "+255712000009" {
		t.Fatalf("create: %+v replayed=%v err=%v", p, replayed, err)
	}
	again, replayed, err := f.svc.Create(ctx, f.driver, "key-first-payin", decimal.NewFromInt(1000), "0712000009")
	if err != nil || !replayed || again.ID != p.ID {
		t.Fatalf("replay: %+v replayed=%v err=%v", again, replayed, err)
	}
	if _, _, err := f.svc.Create(ctx, f.driver, "key-second-payin", decimal.NewFromInt(1000), "0712000009"); !errors.Is(err, ErrInProgress) {
		t.Fatalf("second open pay-in: %v", err)
	}
	var count int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM gateway_transactions WHERE payer_id=$1`, f.driver).Scan(&count)
	if count != 1 {
		t.Fatalf("want one row, got %d", count)
	}
}

func TestCollectedPaysDownDebtAndLiftsHold(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if hs, _ := f.holds.Active(ctx, f.driver); len(hs) != 1 {
		t.Fatalf("fixture driver should be on hold: %+v", hs)
	}
	p, _, err := f.svc.Create(ctx, f.driver, "key-collect-ok", decimal.NewFromInt(2000), "0712000009")
	if err != nil {
		t.Fatal(err)
	}
	// Ten concurrent checks (webhook, job and sweep racing) post exactly once.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = f.svc.Refresh(ctx, p.ID) }()
	}
	wg.Wait()
	got, err := f.svc.Get(ctx, f.driver, p.ID)
	if err != nil || got.Status != StatusCollected {
		t.Fatalf("after refresh: %+v %v", got, err)
	}
	var txns int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM ledger_transactions WHERE kind='payin.collected' AND reference=$1`, p.ID).Scan(&txns)
	if txns != 1 {
		t.Fatalf("want one ledger transaction, got %d", txns)
	}
	if d := f.debt(t); !d.Equal(decimal.NewFromInt(4000)) {
		t.Fatalf("debt after payment: %s", d)
	}
	if hs, _ := f.holds.Active(ctx, f.driver); len(hs) != 0 {
		t.Fatalf("hold should lift under the hard limit: %+v", hs)
	}
	if broken, _, err := f.ledger.Verify(ctx); err != nil || broken != 0 {
		t.Fatalf("ledger chain: broken=%d err=%v", broken, err)
	}
}

func TestFailuresPostNothing(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, _, err := f.svc.Create(ctx, f.driver, "key-no-funds", decimal.NewFromInt(1000), "0712000001")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Refresh(ctx, p.ID)
	if got.Status != StatusFailed || *got.FailureCode != payment.FailInsufficientFunds {
		t.Fatalf("insufficient funds: %+v", got)
	}
	// The failed row is closed, so a new attempt is allowed; this one never completes and expires.
	p2, _, err := f.svc.Create(ctx, f.driver, "key-never", decimal.NewFromInt(1000), "0712000003")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ = f.svc.Refresh(ctx, p2.ID); got.Status != StatusPending {
		t.Fatalf("should still be pending: %+v", got)
	}
	f.now = f.now.Add(ExpireAfter + time.Second)
	if got, _ = f.svc.Refresh(ctx, p2.ID); got.Status != StatusFailed || *got.FailureCode != FailExpired {
		t.Fatalf("should expire: %+v", got)
	}
	if _, _, err := f.svc.Create(ctx, f.driver, "key-gateway-down", decimal.NewFromInt(1000), "0712000004"); !errors.Is(err, ErrGatewayDown) {
		t.Fatalf("gateway down: %v", err)
	}
	if d := f.debt(t); !d.Equal(decimal.NewFromInt(6000)) {
		t.Fatalf("debt must be unchanged, got %s", d)
	}
}

func TestWebhookCannotFakeSuccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, _, err := f.svc.Create(ctx, f.driver, "key-webhook-lie", decimal.NewFromInt(1000), "0712000003")
	if err != nil {
		t.Fatal(err)
	}
	f.svc.Webhook(ctx, p.ID) // a callback claiming anything only triggers a gateway check
	if got, _ := f.svc.Get(ctx, f.driver, p.ID); got.Status != StatusPending {
		t.Fatalf("gateway says pending, so the row stays pending: %+v", got)
	}
	if _, err := f.svc.Get(ctx, "00000000-0000-0000-0000-000000000000", p.ID); !errors.Is(err, ErrPayinNotFound) {
		t.Fatalf("another driver must not see it: %v", err)
	}
}

func TestLostReplyIsStillReconciled(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// Gateway "down" at push time: the driver is told at once, but the reference was stored first.
	_, _, err := f.svc.Create(ctx, f.driver, "key-lost-reply", decimal.NewFromInt(1000), "0712000004")
	if !errors.Is(err, ErrGatewayDown) {
		t.Fatalf("want GATEWAY_UNAVAILABLE, got %v", err)
	}
	var ref *string
	var code string
	_ = f.pool.QueryRow(ctx, `SELECT provider_ref, failure_code FROM gateway_transactions WHERE payer_id=$1`, f.driver).Scan(&ref, &code)
	if ref == nil || code != payment.FailGateway {
		t.Fatalf("row must keep its reference for a late re-check: ref=%v code=%s", ref, code)
	}
	// The reconciliation sweep re-checks it; the fake says the gateway really failed, so nothing is credited.
	if err := f.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if d := f.debt(t); !d.Equal(decimal.NewFromInt(6000)) {
		t.Fatalf("debt must be unchanged, got %s", d)
	}
}

func TestPushesAreRateLimited(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i := 0; i < PushesPerDriverPerHour; i++ {
		// Each prompt to a different number fails fast (insufficient funds), freeing the next attempt.
		phone := "07120000" + string(rune('1'+i)) + "1"
		p, _, err := f.svc.Create(ctx, f.driver, "key-rate-"+phone, decimal.NewFromInt(1000), phone)
		if err != nil {
			t.Fatalf("push %d: %v", i, err)
		}
		if _, err := f.svc.Refresh(ctx, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := f.svc.Create(ctx, f.driver, "key-rate-over", decimal.NewFromInt(1000), "0712000099"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("sixth push in an hour must be refused, got %v", err)
	}
}

func TestWebhookOnlyActsOnOpenPayins(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// A made-up id does nothing (no job, no error); a real pending pay-in is re-checked at once.
	f.svc.Webhook(ctx, "6f1c0b4e-0000-4000-8000-000000000000")
	p, _, err := f.svc.Create(ctx, f.driver, "key-webhook-open", decimal.NewFromInt(1000), "0712000009")
	if err != nil {
		t.Fatal(err)
	}
	f.svc.Webhook(ctx, p.ID) // no job client in tests: refreshes inline, and the fake settles immediately
	if got, _ := f.svc.Get(ctx, f.driver, p.ID); got.Status != StatusCollected {
		t.Fatalf("webhook on an open pay-in must re-check it: %+v", got)
	}
	f.svc.Webhook(ctx, p.ID) // settled: nothing more happens
}
