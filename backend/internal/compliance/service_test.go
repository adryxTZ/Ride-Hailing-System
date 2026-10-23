package compliance

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator/fake"
)

// countingRegulator counts calls per kind around the fake adapter.
type countingRegulator struct {
	*fake.Regulator
	mu    sync.Mutex
	calls map[string]int
}

func (c *countingRegulator) count(kind string) {
	c.mu.Lock()
	c.calls[kind]++
	c.mu.Unlock()
}

func (c *countingRegulator) RegisterDriver(ctx context.Context, d regulator.DriverRecord) (regulator.Ack, error) {
	c.count(KindDriver)
	return c.Regulator.RegisterDriver(ctx, d)
}

func (c *countingRegulator) RegisterVehicle(ctx context.Context, v regulator.VehicleRecord) (regulator.Ack, error) {
	c.count(KindVehicle)
	return c.Regulator.RegisterVehicle(ctx, v)
}

type fixture struct {
	svc             *Service
	reg             *countingRegulator
	pool            *pgxpool.Pool
	driver, vehicle string
	country         string
}

// newFixture uses a test country code ("ZZ") so real rows are never touched. Needs NEXA_TEST_DATABASE_URL.
func newFixture(t *testing.T) *fixture {
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
	var brandID string
	if err := pool.QueryRow(ctx, `SELECT id FROM brands ORDER BY created_at LIMIT 1`).Scan(&brandID); err != nil {
		t.Skip("no seeded brand")
	}
	f := &fixture{pool: pool, country: "ZZ", reg: &countingRegulator{Regulator: fake.New("ok"), calls: map[string]int{}}}
	phone := fmt.Sprintf("+2557%05d%04d", rand.IntN(100000), rand.IntN(10000))
	if err := pool.QueryRow(ctx, `INSERT INTO users (brand_id, phone, full_name) VALUES ($1,$2,'Reg Test') RETURNING id`, brandID, phone).Scan(&f.driver); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO driver_profiles (user_id, brand_id, country_code, licence_number, status) VALUES ($1,$2,'TZ','1234567890','APPROVED')`, f.driver, brandID)
	if err := pool.QueryRow(ctx, `INSERT INTO vehicles (brand_id, driver_id, service_key, class_key, plate, status) VALUES ($1,$2,'taxi','standard',$3,'APPROVED') RETURNING id`,
		brandID, f.driver, fmt.Sprintf("T%06d", rand.IntN(1000000))).Scan(&f.vehicle); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM regulator_submissions WHERE country_code='ZZ'`)
		_, _ = pool.Exec(ctx, `DELETE FROM regulator_settings WHERE country_code='ZZ'`)
		_, _ = pool.Exec(ctx, `DELETE FROM vehicles WHERE id=$1`, f.vehicle)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_profiles WHERE user_id=$1`, f.driver)
	})
	countries := []brand.Country{{Code: "ZZ", Regulator: "latra", Timezone: "Africa/Dar_es_Salaam", Currency: "TZS", Status: brand.CountryActive, LicencePattern: `^\d{10}$`}}
	f.svc = New(Deps{Pool: pool, BrandID: brandID, Countries: countries, Regulators: map[string]regulator.Regulator{"latra": f.reg}})
	return f
}

func (f *fixture) enable(t *testing.T, on bool) {
	t.Helper()
	if _, err := f.svc.SetSettings(context.Background(), f.driver, f.country, &on, nil, "test"); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) status(t *testing.T, kind, key string) string {
	t.Helper()
	st, err := f.svc.store.statusOf(context.Background(), "latra", kind, key)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestApprovalEnqueuesInItsTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	rollback := errors.New("rolled back")
	_ = pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		if err := f.svc.EnqueueApprovalTx(ctx, tx, f.country, f.driver, f.vehicle); err != nil {
			t.Fatal(err)
		}
		return rollback
	})
	if st := f.status(t, KindDriver, f.driver); st != "" {
		t.Fatalf("a rolled-back approval queues nothing, got %q", st)
	}
	if err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error { return f.svc.EnqueueApprovalTx(ctx, tx, f.country, f.driver, f.vehicle) }); err != nil {
		t.Fatal(err)
	}
	if f.status(t, KindDriver, f.driver) != StatusPending || f.status(t, KindVehicle, f.vehicle) != StatusPending {
		t.Fatal("driver and vehicle must be queued")
	}
	if err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error { return f.svc.EnqueueApprovalTx(ctx, tx, "KE", f.driver, f.vehicle) }); err != nil {
		t.Fatal("a country without a regulator queues nothing, without error")
	}
}

func TestNothingIsSentWhileOffThenEachRowOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_ = pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error { return f.svc.EnqueueApprovalTx(ctx, tx, f.country, f.driver, f.vehicle) })
	if err := f.svc.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.reg.calls) != 0 {
		t.Fatalf("switched off: nothing is sent, got %v", f.reg.calls)
	}
	f.enable(t, true)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = f.svc.Sweep(ctx) }()
	}
	wg.Wait()
	if f.reg.calls[KindDriver] != 1 || f.reg.calls[KindVehicle] != 1 {
		t.Fatalf("10 concurrent sweeps send each row once, got %v", f.reg.calls)
	}
	if f.status(t, KindDriver, f.driver) != StatusAccepted {
		t.Fatal("accepted")
	}
}

func TestFailClosedGate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	on := true
	if err := f.svc.Gate(ctx, f.country, f.driver, f.vehicle); err != nil {
		t.Fatal("fail-closed off: nobody is stopped")
	}
	if _, err := f.svc.SetSettings(ctx, f.driver, f.country, &on, &on, "test"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Gate(ctx, f.country, f.driver, f.vehicle); !errors.Is(err, ErrRegistrationOpen) {
		t.Fatalf("unregistered driver must wait: %v", err)
	}
	_ = pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error { return f.svc.EnqueueApprovalTx(ctx, tx, f.country, f.driver, f.vehicle) })
	_ = f.svc.Sweep(ctx)
	if err := f.svc.Gate(ctx, f.country, f.driver, f.vehicle); err != nil {
		t.Fatalf("registered driver may go online: %v", err)
	}
	pending := New(Deps{Pool: f.pool, BrandID: f.svc.store.brandID, Countries: f.svc.countries})
	if _, err := pending.SetSettings(ctx, f.driver, f.country, nil, &on, "test"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("fail-closed needs a real adapter: %v", err)
	}
}

func TestLocalInvalidIsNotSent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, _ = f.pool.Exec(ctx, `UPDATE driver_profiles SET licence_number='DL-1' WHERE user_id=$1`, f.driver)
	_ = pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error { return f.svc.EnqueueApprovalTx(ctx, tx, f.country, f.driver, "") })
	f.enable(t, true)
	_ = f.svc.Sweep(ctx)
	var code string
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(last_code,'') FROM regulator_submissions WHERE country_code='ZZ' AND kind='DRIVER'`).Scan(&code)
	if f.reg.calls[KindDriver] != 0 || code != codeLocalInvalid {
		t.Fatalf("a licence breaking the country's rule is held locally: calls %v code %q", f.reg.calls, code)
	}
}
