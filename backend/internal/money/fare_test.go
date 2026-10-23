package money

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// fareFixture needs the compose database (NEXA_TEST_DATABASE_URL), migrated and seeded.
type fareFixture struct {
	pool          *pgxpool.Pool
	ledger        *Ledger
	driver, rider string
}

func newFareFixture(t *testing.T) *fareFixture {
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
	var brandID string
	if err := pool.QueryRow(ctx, `SELECT id FROM brands ORDER BY created_at LIMIT 1`).Scan(&brandID); err != nil {
		t.Skip("no seeded brand")
	}
	f := &fareFixture{pool: pool, ledger: NewLedger(pool, brandID)}
	stamp := time.Now().Format("0405.000000")[0:9]
	for i, id := range []*string{&f.driver, &f.rider} {
		phone := "+2556" + string(rune('0'+i)) + stamp[1:]
		if err := pool.QueryRow(ctx, `INSERT INTO users (brand_id, phone, full_name) VALUES ($1,$2,'Fare Test') RETURNING id`, brandID, phone).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	// Ledger rows are append-only by design; the test accounts stay but touch nobody else.
	return f
}

// split10k is the default policy on a 10,000 fare (see split_test.go).
func split10k(t *testing.T) json.RawMessage {
	t.Helper()
	b, err := Split(d("10000"), d("0"), "TZS", Policy{CommissionRate: d("0.10"), VATRate: d("0.18"), VATBase: "COMMISSION", LevyBorneBy: "PLATFORM"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(b)
	return raw
}

func (f *fareFixture) post(t *testing.T, fn func(tx pgx.Tx) (bool, error)) (bool, error) {
	t.Helper()
	var posted bool
	err := pgx.BeginFunc(context.Background(), f.pool, func(tx pgx.Tx) error {
		var err error
		posted, err = fn(tx)
		return err
	})
	return posted, err
}

func TestMobileMoneyTripCreditsTheDriverAndClearsDues(t *testing.T) {
	f := newFareFixture(t)
	ctx := context.Background()
	trip := "fare-test-" + f.driver
	fare := DigitalFare{TripID: trip, RiderID: f.rider, DriverID: f.driver, Fees: split10k(t), Dues: d("500"), Currency: "TZS", Collected: d("10500")}
	posted, err := f.post(t, func(tx pgx.Tx) (bool, error) { return f.ledger.PostTripPaidTx(ctx, tx, fare) })
	if err != nil || !posted {
		t.Fatalf("post: %v %v", posted, err)
	}
	again, err := f.post(t, func(tx pgx.Tx) (bool, error) { return f.ledger.PostTripPaidTx(ctx, tx, fare) })
	if err != nil || again {
		t.Fatalf("a second posting must be a no-op: %v %v", again, err)
	}
	w, err := f.ledger.DriverWallet(ctx, f.driver, 5)
	if err != nil || !w.Balance.Equal(d("9000")) {
		t.Fatalf("driver should be owed their 9,000 earnings, got %s (%v)", w.Balance, err)
	}
	var clearing, dues decimal.Decimal
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(sum(e.amount) FILTER (WHERE a.kind='GATEWAY_CLEARING'),0), coalesce(sum(e.amount) FILTER (WHERE a.kind='RIDER_DUES'),0)
		FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id JOIN ledger_transactions x ON x.id=e.transaction_id WHERE x.reference=$1`, trip).Scan(&clearing, &dues)
	if !clearing.Equal(d("10500")) || !dues.Equal(d("-500")) {
		t.Fatalf("clearing %s dues %s", clearing, dues)
	}
}

func TestCollectedAmountMustMatchTheFare(t *testing.T) {
	f := newFareFixture(t)
	fare := DigitalFare{TripID: "fare-mismatch-" + f.driver, RiderID: f.rider, DriverID: f.driver, Fees: split10k(t), Currency: "TZS", Collected: d("9999")}
	_, err := f.post(t, func(tx pgx.Tx) (bool, error) { return f.ledger.PostTripPaidTx(context.Background(), tx, fare) })
	if !errors.Is(err, ErrFareMismatch) {
		t.Fatalf("want ErrFareMismatch, got %v", err)
	}
}

func TestLateFareMoneyIsOwedToTheRider(t *testing.T) {
	f := newFareFixture(t)
	ctx := context.Background()
	payin := "payin-late-" + f.rider
	if _, err := f.post(t, func(tx pgx.Tx) (bool, error) {
		return f.ledger.PostFareUnappliedTx(ctx, tx, payin, f.rider, d("7000"), "TZS")
	}); err != nil {
		t.Fatal(err)
	}
	var owed decimal.Decimal
	_ = f.pool.QueryRow(ctx, `SELECT -sum(e.amount) FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE a.kind='RIDER_REFUNDS' AND a.owner_id=$1`, f.rider).Scan(&owed)
	if !owed.Equal(d("7000")) {
		t.Fatalf("rider should be owed 7,000, got %s", owed)
	}
	if seq, _, err := f.ledger.Verify(ctx); err != nil || seq != 0 {
		t.Fatalf("chain broken at %d (%v)", seq, err)
	}
}
