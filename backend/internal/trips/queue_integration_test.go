package trips

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Trip queue against a real database (NEXA_TEST_DATABASE_URL, migrated): a
// finishing driver's next trip is queued, cannot start, is promoted when the
// trip ahead completes, and goes back to matching when released or cancelled.

type queueFixture struct {
	t      *testing.T
	ctx    context.Context
	pool   *pgxpool.Pool
	svc    *Service
	brand  string
	driver string
	users  []string
	trips  []string
}

func newQueueFixture(t *testing.T) *queueFixture {
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
	f := &queueFixture{t: t, ctx: ctx, pool: pool}
	if err := pool.QueryRow(ctx, `SELECT id FROM brands ORDER BY created_at LIMIT 1`).Scan(&f.brand); err != nil {
		t.Skip("no seeded brand")
	}
	f.svc = New(pool, f.brand, nil)
	f.driver = f.user()
	t.Cleanup(f.cleanup)
	return f
}

// user creates a throwaway account with a random test number.
func (f *queueFixture) user() string {
	var id string
	phone := fmt.Sprintf("+2557%08d", rand.IntN(1e8))
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO users (brand_id, phone) VALUES ($1, $2) RETURNING id`, f.brand, phone).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	f.users = append(f.users, id)
	return id
}

// trip inserts a cash ride in status st for a new rider (driver set unless OFFERED).
func (f *queueFixture) trip(st Status) string {
	var id string
	var driver *string
	if st != Offered {
		driver = &f.driver
	}
	err := f.pool.QueryRow(f.ctx, `INSERT INTO trips (brand_id, code, service_key, vehicle_class_key, rider_id, driver_id, lifecycle, quote_option,
		  estimated_fare, currency, country_code, status, payment_status, accepted_at)
		VALUES ($1, upper(substr(md5(random()::text), 1, 8)), 'taxi', 'standard', $2, $3, '{"profile":"pickup_dropoff","payment_at_end":true}', '{}',
		  5000, 'TZS', 'TZ', $4::trip_status, CASE WHEN $4 = 'AWAITING_PAYMENT' THEN 'DUE' END, CASE WHEN $3::uuid IS NULL THEN NULL ELSE now() END)
		RETURNING id`, f.brand, f.user(), driver, string(st)).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	for seq, kind := range []string{"PICKUP", "DROPOFF"} {
		if _, err := f.pool.Exec(f.ctx, `INSERT INTO trip_stops (trip_id, seq, kind, location) VALUES ($1, $2, $3, ST_GeogFromText('SRID=4326;POINT(39.229 -6.7764)'))`,
			id, seq+1, kind); err != nil {
			f.t.Fatal(err)
		}
	}
	f.trips = append(f.trips, id)
	return id
}

// assignNext accepts an offered trip for the fixture driver.
func (f *queueFixture) assignNext(tripID string) (bool, error) {
	var queued bool
	err := pgx.BeginFunc(f.ctx, f.pool, func(tx pgx.Tx) error {
		var err error
		queued, err = f.svc.AssignNext(f.ctx, tx, tripID, f.driver, 300)
		return err
	})
	return queued, err
}

func (f *queueFixture) load(id string) *Trip {
	t, err := f.svc.Load(f.ctx, id)
	if err != nil || t == nil {
		f.t.Fatalf("load %s: %v", id, err)
	}
	return t
}

func (f *queueFixture) cleanup() {
	for _, q := range []string{`DELETE FROM trip_events WHERE trip_id = ANY($1::uuid[])`, `DELETE FROM trip_stops WHERE trip_id = ANY($1::uuid[])`,
		`UPDATE trips SET queued_after = NULL WHERE id = ANY($1::uuid[])`, `DELETE FROM trips WHERE id = ANY($1::uuid[])`} {
		_, _ = f.pool.Exec(f.ctx, q, f.trips)
	}
	_, _ = f.pool.Exec(f.ctx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, f.users)
	f.pool.Close()
}

func TestQueuedTripWaitsAndIsPromotedOnCompletion(t *testing.T) {
	f := newQueueFixture(t)
	current, next := f.trip(AwaitingPayment), f.trip(Offered)
	if queued, err := f.assignNext(next); err != nil || !queued {
		t.Fatalf("accept while finishing: queued=%v err=%v", queued, err)
	}
	if b := f.load(next); !b.Queued || b.DriverID == nil || b.Status != Accepted || slices.Contains(b.AllowedNext, EnRouteToPickup) {
		t.Fatalf("next trip: %+v", b)
	}
	if a, _ := f.svc.ActiveForDriver(f.ctx, f.driver); a == nil || a.ID != current {
		t.Fatalf("the current trip stays the driver's active one, got %v", a)
	}
	// One queued trip at most.
	if _, err := f.assignNext(f.trip(Offered)); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("second next trip: want NEXT_TRIP_ALREADY_QUEUED, got %v", err)
	}
	// The next trip cannot start before the current one completes.
	if _, err := f.svc.TransitionByDriver(f.ctx, next, f.driver, EnRouteToPickup, ""); !errors.Is(err, ErrQueuedTripWaiting) {
		t.Fatalf("start before completion: want NEXT_TRIP_WAITING, got %v", err)
	}
	// Cash confirmation of the current trip is not blocked by the queued one.
	if done, err := f.svc.TransitionByDriver(f.ctx, current, f.driver, Completed, ""); err != nil || done.Status != Completed {
		t.Fatalf("complete current: %v", err)
	}
	b := f.load(next)
	if b.Queued || !slices.Contains(b.AllowedNext, EnRouteToPickup) {
		t.Fatalf("promoted trip: queued=%v next=%v", b.Queued, b.AllowedNext)
	}
	if a, _ := f.svc.ActiveForDriver(f.ctx, f.driver); a == nil || a.ID != next {
		t.Fatalf("the next trip is active after completion, got %v", a)
	}
	if _, err := f.svc.TransitionByDriver(f.ctx, next, f.driver, EnRouteToPickup, ""); err != nil {
		t.Fatalf("start after completion: %v", err)
	}
}

func TestAcceptAfterTheTripAheadEndedIsNotQueued(t *testing.T) {
	f := newQueueFixture(t)
	if queued, err := f.assignNext(f.trip(Offered)); err != nil || queued {
		t.Fatalf("a free driver gets a current trip: queued=%v err=%v", queued, err)
	}
}

func TestReleasedQueuedTripGoesBackToMatching(t *testing.T) {
	f := newQueueFixture(t)
	f.trip(ArrivedAtDropoff)
	next := f.trip(Offered)
	if _, err := f.assignNext(next); err != nil {
		t.Fatal(err)
	}
	released, err := f.svc.ReleaseQueued(f.ctx, next, f.driver)
	if err != nil || !released {
		t.Fatalf("release: %v %v", released, err)
	}
	if b := f.load(next); b.Status != Requested || b.DriverID != nil || b.Queued {
		t.Fatalf("released trip: %+v", b)
	}
	if again, _ := f.svc.ReleaseQueued(f.ctx, next, f.driver); again {
		t.Fatal("releasing twice must be a no-op")
	}
}

func TestDriverCancelOfQueuedTripRedispatches(t *testing.T) {
	f := newQueueFixture(t)
	f.trip(AwaitingPayment)
	next := f.trip(Offered)
	if _, err := f.assignNext(next); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.TransitionByDriver(f.ctx, next, f.driver, CancelledByDriver, ""); err != nil {
		t.Fatal(err)
	}
	if b := f.load(next); b.Status != Requested || b.DriverID != nil || b.QueuedAfter != nil {
		t.Fatalf("cancelled queued trip: %+v", b)
	}
	if n, _ := f.svc.QueuedForDriver(f.ctx, f.driver); n != nil {
		t.Fatal("the driver has no queued trip after cancelling it")
	}
}

func TestStaffCancelOfTripAheadPromotesTheNext(t *testing.T) {
	f := newQueueFixture(t)
	current, next := f.trip(ArrivedAtDropoff), f.trip(Offered)
	if _, err := f.assignNext(next); err != nil {
		t.Fatal(err)
	}
	staff := f.user()
	if _, err := f.svc.AdminCancel(f.ctx, staff, current, "test"); err != nil {
		t.Fatal(err)
	}
	_, _ = f.pool.Exec(f.ctx, `DELETE FROM audit_log WHERE actor_id=$1`, staff)
	if b := f.load(next); b.Queued {
		t.Fatal("a staff cancel of the trip ahead promotes the next trip")
	}
}
