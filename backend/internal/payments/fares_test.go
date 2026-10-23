package payments

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment/fake"
)

// fakeTrips records what payments asks of the trips module and answers as told.
type fakeTrips struct {
	mu        sync.Mutex
	pending   []string
	failed    []string
	completed []string
	settle    *Settlement // nil = the trip no longer waits for this payment
}

func (f *fakeTrips) FarePendingTx(_ context.Context, _ pgx.Tx, _, payinID string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = append(f.pending, payinID)
	return nil
}

func (f *fakeTrips) FareFailed(_ context.Context, _, _, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, code)
	return nil
}

func (f *fakeTrips) CompletePaidTx(_ context.Context, _ pgx.Tx, _, payinID string) (*Settlement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completed = append(f.completed, payinID)
	return f.settle, nil
}

func (f *fakeTrips) Notify(context.Context, string) {}

// fakeFareLedger records postings; err makes the trip posting fail.
type fakeFareLedger struct {
	mu        sync.Mutex
	paid      int
	unapplied []decimal.Decimal
	err       error
}

func (l *fakeFareLedger) PostTripPaidTx(context.Context, pgx.Tx, Settlement, decimal.Decimal) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return false, l.err
	}
	l.paid++
	return true, nil
}

func (l *fakeFareLedger) PostFareUnappliedTx(_ context.Context, _ pgx.Tx, _, _ string, amount decimal.Decimal, _ string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.unapplied = append(l.unapplied, amount)
	return true, nil
}

func (l *fakeFareLedger) WalletMoved(context.Context, string) {}

type fareFixture struct {
	svc    *Service
	trips  *fakeTrips
	ledger *fakeFareLedger
	fare   Fare
	pool   *pgxpool.Pool
}

// newFareFixture uses a real trip with no fare prompts yet (gateway rows reference
// trips) and a new rider. Needs NEXA_TEST_DATABASE_URL with at least one trip.
func newFareFixture(t *testing.T) *fareFixture {
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
	var brandID, tripID, rider string
	if err := pool.QueryRow(ctx, `SELECT t.brand_id, t.id FROM trips t WHERE NOT EXISTS (SELECT 1 FROM gateway_transactions g WHERE g.trip_id=t.id)
		ORDER BY t.created_at DESC LIMIT 1`).Scan(&brandID, &tripID); err != nil {
		t.Skip("no trip to attach fare prompts to")
	}
	phone := fmt.Sprintf("+2557%05d%04d", rand.IntN(100000), 9000+rand.IntN(999))
	if err := pool.QueryRow(ctx, `INSERT INTO users (brand_id, phone, full_name) VALUES ($1,$2,'Fare Test') RETURNING id`, brandID, phone).Scan(&rider); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM audit_log WHERE target=$1`, "trip:"+tripID)
		_, _ = pool.Exec(ctx, `DELETE FROM gateway_transactions WHERE payer_id=$1`, rider)
	})
	f := &fareFixture{trips: &fakeTrips{}, ledger: &fakeFareLedger{}, pool: pool,
		fare: Fare{TripID: tripID, RiderID: rider, Phone: phone, Amount: decimal.NewFromInt(6400), Currency: "TZS"}}
	f.trips.settle = &Settlement{TripID: tripID, RiderID: rider, DriverID: rider, Currency: "TZS"}
	gw := &fake.Gateway{Delay: 0, Now: time.Now}
	f.svc = New(Deps{Pool: pool, BrandID: brandID, Country: "TZ", Gateways: payment.NewGateways(map[string]payment.Gateway{"TZS": gw}), Fares: f.trips, FareLedger: f.ledger})
	return f
}

func TestCollectedFareCompletesTheTripAndPostsOnce(t *testing.T) {
	f := newFareFixture(t)
	ctx := context.Background()
	p, err := f.svc.PromptFare(ctx, f.fare, "", "fare-key-0001", true)
	if err != nil || p.Status != StatusPending || len(f.trips.pending) != 1 {
		t.Fatalf("prompt: %+v %v %v", p, err, f.trips.pending)
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ { // webhook, job and sweep racing
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = f.svc.Refresh(ctx, p.ID) }()
	}
	wg.Wait()
	if f.ledger.paid != 1 || len(f.trips.completed) != 1 {
		t.Fatalf("one completion and one posting: paid %d completed %d", f.ledger.paid, len(f.trips.completed))
	}
}

func TestMoneyForATripThatNoLongerWaitsIsOwedBack(t *testing.T) {
	f := newFareFixture(t)
	f.trips.settle = nil // the rider paid cash meanwhile
	ctx := context.Background()
	p, _ := f.svc.PromptFare(ctx, f.fare, "", "", true)
	if _, err := f.svc.Refresh(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if f.ledger.paid != 0 || len(f.ledger.unapplied) != 1 || !f.ledger.unapplied[0].Equal(f.fare.Amount) {
		t.Fatalf("owed back: %+v", f.ledger)
	}
}

func TestSettlementFailureNeverLeavesMoneyStuck(t *testing.T) {
	f := newFareFixture(t)
	f.ledger.err = errors.New("split disagrees with the amount")
	ctx := context.Background()
	p, _ := f.svc.PromptFare(ctx, f.fare, "", "", true)
	got, err := f.svc.Refresh(ctx, p.ID)
	if err != nil || got.Status != StatusCollected {
		t.Fatalf("the row must close: %+v %v", got, err)
	}
	if len(f.ledger.unapplied) != 1 || len(f.trips.failed) != 1 || f.trips.failed[0] != FailSettlement {
		t.Fatalf("owed back and the trip freed: %+v %+v", f.ledger, f.trips.failed)
	}
}

func TestPromptCapAndKeys(t *testing.T) {
	f := newFareFixture(t)
	ctx := context.Background()
	refuse := f.fare
	for i := 0; i < MaxFarePrompts; i++ {
		p, err := f.svc.PromptFare(ctx, refuse, "0712000001", fmt.Sprintf("cap-key-%d", i), false) // insufficient funds
		if err != nil {
			t.Fatalf("prompt %d: %v", i, err)
		}
		if _, err := f.svc.Refresh(ctx, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.PromptFare(ctx, refuse, "", "cap-key-9", false); !errors.Is(err, ErrFarePromptLimit) {
		t.Fatalf("a fourth prompt is refused: %v", err)
	}
	again, err := f.svc.PromptFare(ctx, refuse, "", "cap-key-0", false)
	if err != nil || again == nil {
		t.Fatalf("a replayed key answers with its prompt: %v", err)
	}
	other := refuse
	other.TripID = "00000000-0000-0000-0000-000000000000"
	if _, err := f.svc.ReplayFare(ctx, other.RiderID, other.TripID, "cap-key-0"); !errors.Is(err, ErrKeyReused) {
		t.Fatalf("a key from another trip is refused: %v", err)
	}
	if len(f.trips.failed) != MaxFarePrompts {
		t.Fatalf("each failure reaches the trip: %v", f.trips.failed)
	}
}

func TestNoGatewayInACountryMeansCash(t *testing.T) {
	f := newFareFixture(t)
	kes := f.fare
	kes.Currency = "KES" // no provider configured for this country
	if _, err := f.svc.PromptFare(context.Background(), kes, "", "", false); !errors.Is(err, ErrGatewayDown) {
		t.Fatalf("no gateway in the country: %v", err)
	}
	fellBack := false
	f.svc.AutoPrompt(context.Background(), kes, func(context.Context) { fellBack = true })
	if !fellBack {
		t.Fatal("a mobile-money ride in a country without a gateway falls back to cash")
	}
}
