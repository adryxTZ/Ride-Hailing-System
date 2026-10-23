package holds

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
)

// Integration tests need the compose database (NEXA_TEST_DATABASE_URL), migrated and seeded.
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
	if err := pool.QueryRow(context.Background(), `SELECT id FROM brands ORDER BY created_at LIMIT 1`).Scan(&brandID); err != nil {
		t.Skip("no seeded brand")
	}
	return pool, brandID
}

// newDriver inserts a throwaway driver and removes it afterwards.
func newDriver(t *testing.T, pool *pgxpool.Pool, brandID string) string {
	t.Helper()
	var id string
	phone := "+2557" + time.Now().Format("0405.000000")[0:9]
	if err := pool.QueryRow(context.Background(), `INSERT INTO users (brand_id, phone, full_name) VALUES ($1,$2,'Test Driver') RETURNING id`, brandID, phone).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO driver_profiles (user_id, brand_id, country_code) SELECT $1, $2, country FROM brands WHERE id=$2`, id, brandID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM audit_log WHERE target=$1`, "driver:"+id)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_holds WHERE driver_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_debt_clock WHERE driver_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_profiles WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	})
	return id
}

type fakeDebts struct{ debt decimal.Decimal }

func (f *fakeDebts) DriverDebtIn(context.Context, pgx.Tx, string) (decimal.Decimal, string, error) {
	return f.debt, "TZS", nil
}

type fakePresence struct {
	state   presence.State
	online  bool
	offline int
}

func (f *fakePresence) Get(context.Context, string) (*presence.Driver, error) {
	if !f.online {
		return nil, nil
	}
	return &presence.Driver{State: f.state}, nil
}
func (f *fakePresence) Offline(context.Context, string) error {
	f.offline++
	f.online = false
	return nil
}

var testLimits = Limits{Soft: decimal.NewFromInt(1000), Hard: decimal.NewFromInt(5000), GraceDays: 7, MinPayin: decimal.NewFromInt(1000)}

func newService(t *testing.T) (*Service, string, *fakeDebts, *fakePresence) {
	t.Helper()
	pool, brandID := testPool(t)
	driver := newDriver(t, pool, brandID)
	debts := &fakeDebts{}
	pres := &fakePresence{state: presence.Idle, online: true}
	svc := New(pool, brandID, debts, func(context.Context) (Limits, error) { return testLimits, nil }, pres, nil)
	return svc, driver, debts, pres
}

func TestEvaluatePlacesAndLiftsDebtHold(t *testing.T) {
	svc, driver, debts, pres := newService(t)
	ctx := context.Background()
	debts.debt = decimal.NewFromInt(5000)
	d, err := svc.Evaluate(ctx, driver)
	if err != nil || !d.Held || d.Cause != CauseHardLimit {
		t.Fatalf("hard limit: %+v %v", d, err)
	}
	if pres.offline != 1 {
		t.Fatalf("held driver should be taken offline once, got %d", pres.offline)
	}
	var actions int
	_ = svc.store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE target=$1 AND action=$2 AND actor_id IS NULL`, "driver:"+driver, actionDebtPlace).Scan(&actions)
	if actions != 1 {
		t.Fatalf("want one system audit entry, got %d", actions)
	}
	debts.debt = decimal.NewFromInt(900)
	if d, err = svc.Evaluate(ctx, driver); err != nil || d.Held {
		t.Fatalf("paid down: %+v %v", d, err)
	}
	if hs, _ := svc.Active(ctx, driver); len(hs) != 0 {
		t.Fatalf("hold should be lifted, got %+v", hs)
	}
}

func TestOnTripDriverIsNotKickedOff(t *testing.T) {
	svc, driver, debts, pres := newService(t)
	ctx := context.Background()
	pres.state = presence.OnTrip
	debts.debt = decimal.NewFromInt(6000)
	if _, err := svc.Evaluate(ctx, driver); err != nil {
		t.Fatal(err)
	}
	if pres.offline != 0 {
		t.Fatal("driver on a trip must finish it")
	}
	pres.state = presence.Idle
	if err := svc.Enforce(ctx, driver); err != nil || pres.offline != 1 {
		t.Fatalf("enforce after trip: offline=%d err=%v", pres.offline, err)
	}
}

func TestManualHoldIsIndependent(t *testing.T) {
	svc, driver, debts, _ := newService(t)
	ctx := context.Background()
	staff := driver // any user id works as the actor in tests
	if _, err := svc.Place(ctx, staff, driver, ""); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("want REASON_REQUIRED, got %v", err)
	}
	h, err := svc.Place(ctx, staff, driver, "Complaint under investigation")
	if err != nil || h.Kind != KindManual {
		t.Fatalf("place: %+v %v", h, err)
	}
	if _, err := svc.Place(ctx, staff, driver, "again"); !errors.Is(err, ErrHoldExists) {
		t.Fatalf("second manual hold: %v", err)
	}
	// The debt rule with no debt must leave the manual hold alone.
	debts.debt = decimal.Zero
	if _, err := svc.Evaluate(ctx, driver); err != nil {
		t.Fatal(err)
	}
	if hs, _ := svc.Active(ctx, driver); len(hs) != 1 || hs[0].Kind != KindManual {
		t.Fatalf("manual hold must survive Evaluate: %+v", hs)
	}
	// Releasing the manual hold leaves a debt hold in place.
	debts.debt = decimal.NewFromInt(9000)
	_, _ = svc.Evaluate(ctx, driver)
	left, err := svc.Release(ctx, staff, driver, "Cleared")
	if err != nil || len(left) != 1 || left[0].Kind != KindDebt {
		t.Fatalf("release: left=%+v err=%v", left, err)
	}
	if _, err := svc.Release(ctx, staff, driver, "again"); !errors.Is(err, ErrNoManualHold) {
		t.Fatalf("second release: %v", err)
	}
}

func TestSweepHoldsOverdueSoftDebt(t *testing.T) {
	svc, driver, debts, _ := newService(t)
	ctx := context.Background()
	debts.debt = decimal.NewFromInt(1500)
	if d, _ := svc.Evaluate(ctx, driver); d.Held {
		t.Fatal("not yet overdue")
	}
	// Backdate the clock past the grace period, as if 8 days went by with no trips.
	if _, err := svc.store.pool.Exec(ctx, `UPDATE driver_debt_clock SET over_soft_since=now() - interval '8 days' WHERE driver_id=$1`, driver); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.Sweep(ctx); err != nil || n == 0 {
		t.Fatalf("sweep: n=%d err=%v", n, err)
	}
	hs, _ := svc.Active(ctx, driver)
	if len(hs) != 1 || hs[0].Cause != CauseSoftLimitOverdue {
		t.Fatalf("want an overdue soft hold, got %+v", hs)
	}
	st, err := svc.Status(ctx, driver)
	if err != nil || st.PayToUnhold == nil || !st.PayToUnhold.Equal(decimal.NewFromInt(1000)) {
		t.Fatalf("status: %+v %v", st, err)
	}
}
