package accounts

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
)

// dbFixture is a fresh brand (ledger rows are append-only, so it stays) with
// a rider, a payee, a completed trip and two refunds on it.
type dbFixture struct {
	pool             *pgxpool.Pool
	ledger           *money.Ledger
	svc              *Service
	brand, rider     string
	payee, trip      string
	refund1, refund2 string
}

const testCurrency = "TZS"

func newDBFixture(t *testing.T) *dbFixture {
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
	f := &dbFixture{pool: pool}
	n := rand.IntN(1_000_000)
	must(t, pool.QueryRow(ctx, `INSERT INTO brands (code, display_name, country, currency, timezone, default_locale)
		VALUES ($1, 'Test brand', 'ZZ', $2, 'UTC', 'en') RETURNING id`, fmt.Sprintf("test-acc-%d", n), testCurrency).Scan(&f.brand))
	must(t, pool.QueryRow(ctx, `INSERT INTO users (brand_id, phone, full_name) VALUES ($1, $2, 'Accounts Test') RETURNING id`,
		f.brand, fmt.Sprintf("+25569%07d", n)).Scan(&f.rider))
	must(t, pool.QueryRow(ctx, `INSERT INTO charge_payees (brand_id, country_code, name, kind) VALUES ($1, 'ZZ', 'Test airport', 'AIRPORT') RETURNING id`,
		f.brand).Scan(&f.payee))
	must(t, pool.QueryRow(ctx, `INSERT INTO trips (brand_id, code, service_key, vehicle_class_key, rider_id, status, lifecycle, quote_option,
		estimated_fare, final_fare, currency, country_code, fees)
		VALUES ($1, $2, 'taxi', 'standard', $3, 'COMPLETED', '{}', '{}', 10000, 10000, $4, 'ZZ', '{"total":"10000"}') RETURNING id`,
		f.brand, fmt.Sprintf("T%d", n), f.rider, testCurrency).Scan(&f.trip))
	f.ledger = money.NewLedger(pool, f.brand)
	f.svc = New(pool, f.brand, f.ledger)
	f.postTrip(t)
	f.refund1, f.refund2 = f.addRefund(t, "300"), f.addRefund(t, "200")
	return f
}

// postTrip books a 10,000 fare paid by mobile money: 9,000 revenue, 1,800 VAT,
// 700 levy, 500 for the payee (gateway 12,000 = the rider paid fare + charges).
func (f *dbFixture) postTrip(t *testing.T) {
	t.Helper()
	legs := []money.Leg{
		{Kind: money.GatewayClearing, Amount: d("12000")},
		{Kind: money.PlatformRevenue, Amount: d("-9000")},
		{Kind: money.VATPayable, Amount: d("-1800")},
		{Kind: money.LevyPayable, Amount: d("-700")},
		{Kind: money.ChargesPayable, OwnerID: f.payee, Amount: d("-500")},
	}
	if _, err := f.ledger.Post(context.Background(), "trip.completed", f.trip, "test trip", testCurrency, legs); err != nil {
		t.Fatal(err)
	}
}

// addRefund records an owed refund borne by the platform, as refunds.Create does.
func (f *dbFixture) addRefund(t *testing.T, amount string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO trip_refunds (brand_id, trip_id, rider_id, amount, currency, borne_by, reason, created_by)
			VALUES ($1, $2, $3, $4, $5, 'PLATFORM', 'test', $3) RETURNING id`, f.brand, f.trip, f.rider, amount, testCurrency).Scan(&id); err != nil {
			return err
		}
		_, err := f.ledger.PostRefundTx(ctx, tx, id, f.rider, "", money.RefundByPlatform, d(amount), testCurrency)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond) // refunds are paid oldest first: keep their order distinct
	return id
}

func (f *dbFixture) pay(key, account, amount string, refunds ...string) (*Payment, error) {
	return f.svc.RecordPayment(context.Background(), PaymentInput{Key: account, Currency: testCurrency, Country: "ZZ", Amount: d(amount),
		PaidOn: time.Now().Format(dateLayout), Method: "BANK_TRANSFER", Reference: "TEST-REF", Reason: "test payment",
		RefundIDs: refunds, IdemKey: key, StaffID: f.rider})
}

func (f *dbFixture) line(t *testing.T, key string) Account {
	t.Helper()
	list, err := f.svc.List(context.Background(), testCurrency, wide())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list.Accounts {
		if a.Key == key {
			return a
		}
	}
	t.Fatalf("no line %s", key)
	return Account{}
}

func wide() Period { return Period{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour)} }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBalancesEqualTheSumOfEntries(t *testing.T) {
	f := newDBFixture(t)
	ctx := context.Background()
	list, err := f.svc.List(ctx, testCurrency, wide())
	must(t, err)
	if !list.Fares.Equal(d("10000")) || list.Trips != 1 {
		t.Errorf("fares %s over %d trips", list.Fares, list.Trips)
	}
	for _, a := range list.Accounts {
		page, err := f.svc.Entries(ctx, a.Key, testCurrency, wide(), EntriesPageMax, 0)
		must(t, err)
		sum := decimal.Zero
		for _, e := range page.Entries {
			sum = sum.Add(e.Amount)
		}
		if !sum.Equal(a.Balance) || page.Total != a.Entries {
			t.Errorf("%s: balance %s, entries sum %s (%d of %d)", a.Key, a.Balance, sum, page.Total, a.Entries)
		}
		if len(page.Entries) > 0 && !page.Entries[0].Running.Equal(a.Balance) {
			t.Errorf("%s: newest running balance %s, balance %s", a.Key, page.Entries[0].Running, a.Balance)
		}
	}
	vat := f.line(t, money.VATPayable)
	if vat.Position != PositionWeOwe || !vat.Balance.Equal(d("1800")) || vat.Rate == nil || !vat.Rate.Equal(d("0.18")) {
		t.Errorf("VAT line %+v", vat)
	}
	if refunds := f.line(t, money.RiderRefunds); !refunds.Balance.Equal(d("500")) || !refunds.WeOwe.Equal(d("500")) {
		t.Errorf("rider refunds line %+v", refunds)
	}
}

func TestPaymentIsBalancedIdempotentAndRefusesOverpay(t *testing.T) {
	f := newDBFixture(t)
	p, err := f.pay("k-vat", money.VATPayable, "1000")
	if err != nil || p.Replayed {
		t.Fatalf("first payment: %+v %v", p, err)
	}
	again, err := f.pay("k-vat", money.VATPayable, "1000")
	if err != nil || !again.Replayed || again.ID != p.ID {
		t.Fatalf("a repeated key must replay: %+v %v", again, err)
	}
	if _, err := f.pay("k-vat", money.VATPayable, "900"); !errors.Is(err, ErrKeyReused) {
		t.Errorf("the key for another payment: %v", err)
	}
	if got := f.line(t, money.VATPayable).Balance; !got.Equal(d("800")) {
		t.Errorf("VAT owed after paying 1,000 once: %s", got)
	}
	if bank := f.line(t, money.PlatformBank).Balance; !bank.Equal(d("-1000")) {
		t.Errorf("bank after one payment: %s", bank)
	}
	if _, err := f.pay("k-vat-2", money.VATPayable, "800.01"); !errors.Is(err, ErrOverpay) {
		t.Errorf("overpaying VAT: %v", err)
	}
	payee := KeyOf(money.ChargesPayable, f.payee)
	if _, err := f.pay("k-payee", payee, "500"); err != nil {
		t.Fatal(err)
	}
	if got := f.line(t, payee).Balance; !got.IsZero() {
		t.Errorf("payee owed after paying in full: %s", got)
	}
	if _, err := f.pay("k-rev", money.PlatformRevenue, "1"); !errors.Is(err, ErrNotSettleable) {
		t.Errorf("revenue is not payable: %v", err)
	}
	f.verify(t)
}

func TestRiderRefundPaymentsMarkRefundsPaid(t *testing.T) {
	f := newDBFixture(t)
	if _, err := f.pay("k-r0", money.RiderRefunds, "400"); !errors.Is(err, ErrAmountMismatch) {
		t.Errorf("a partial refund: %v", err)
	}
	p, err := f.pay("k-r1", money.RiderRefunds, "300")
	if err != nil || len(p.RefundIDs) != 1 || p.RefundIDs[0] != f.refund1 {
		t.Fatalf("oldest first: %+v %v", p, err)
	}
	if _, err := f.pay("k-r2", money.RiderRefunds, "300", f.refund1); !errors.Is(err, ErrRefundNotOwed) {
		t.Errorf("paying a paid refund again: %v", err)
	}
	if _, err := f.pay("k-r3", money.RiderRefunds, "200", f.refund2); err != nil {
		t.Fatal(err)
	}
	var owed int
	must(t, f.pool.QueryRow(context.Background(), `SELECT count(*) FROM trip_refunds WHERE trip_id = $1 AND status = 'OWED'`, f.trip).Scan(&owed))
	if owed != 0 || !f.line(t, money.RiderRefunds).Balance.IsZero() {
		t.Errorf("after paying both: %d owed rows, balance %s", owed, f.line(t, money.RiderRefunds).Balance)
	}
	f.verify(t)
}

func (f *dbFixture) verify(t *testing.T) {
	t.Helper()
	broken, n, err := f.ledger.Verify(context.Background())
	if err != nil || broken != 0 || n == 0 {
		t.Fatalf("chain: broken at %d after %d entries, %v", broken, n, err)
	}
}
