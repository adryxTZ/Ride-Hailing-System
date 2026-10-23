package payouts

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment/fake"
)

var d = decimal.RequireFromString

type fixture struct {
	svc    *Service
	ledger *money.Ledger
	pool   *pgxpool.Pool
	driver string
	staff  string
	gw     *fake.Gateway
	now    time.Time
	rules  Rules
	held   bool
}

// newFixture makes a driver the platform owes `owed`, whose phone ends in suffix
// (the fake gateway's scripted outcome). Needs NEXA_TEST_DATABASE_URL.
func newFixture(t *testing.T, owed, suffix string) *fixture {
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
	f := &fixture{pool: pool, now: time.Now(), rules: Rules{Min: d("5000")}}
	phone := fmt.Sprintf("+2557%05d%s", rand.IntN(100000), suffix)
	for _, id := range []*string{&f.driver, &f.staff} {
		p := phone
		if id == &f.staff {
			p = fmt.Sprintf("+2556%05d%s", rand.IntN(100000), "9999")
		}
		if err := pool.QueryRow(ctx, `INSERT INTO users (brand_id, phone, full_name) VALUES ($1,$2,'Payout Test') RETURNING id`, brandID, p).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM audit_log WHERE target LIKE 'payout:%' AND actor_id=$1`, f.staff)
		_, _ = pool.Exec(ctx, `DELETE FROM driver_payouts WHERE driver_id=$1`, f.driver)
	})
	f.ledger = money.NewLedger(pool, brandID)
	f.gw = &fake.Gateway{Delay: 0, Now: func() time.Time { return f.now }}
	f.svc = New(Deps{Pool: pool, BrandID: brandID, Gateways: payment.NewPayouters(map[string]payment.Payouter{"TZS": f.gw}), Ledger: f.ledger,
		Rules:   func(context.Context) (Rules, error) { return f.rules, nil },
		Blocked: func(context.Context, string) (bool, error) { return f.held, nil },
		Person:  func(context.Context, string) (string, string, error) { return phone, "Payout Test", nil }})
	f.svc.now = func() time.Time { return f.now }
	f.credit(t, owed)
	return f
}

// credit makes the platform owe the driver more (a mobile-money trip's earnings).
func (f *fixture) credit(t *testing.T, amount string) {
	t.Helper()
	_, err := f.ledger.Post(context.Background(), "test.earnings", fmt.Sprintf("earn-%s-%d", f.driver, rand.Int64()), "test earnings", "TZS",
		[]money.Leg{{Kind: money.GatewayClearing, Amount: d(amount)}, {Kind: money.DriverWallet, OwnerID: f.driver, Amount: d(amount).Neg()}})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) owed(t *testing.T) decimal.Decimal {
	t.Helper()
	o, _, err := f.ledger.DriverOwed(context.Background(), f.driver)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func key() string { return fmt.Sprintf("key-%d", rand.Int64()) }

func TestRequestRules(t *testing.T) {
	f := newFixture(t, "12000", "0009")
	ctx := context.Background()
	for name, c := range map[string]struct {
		amount string
		want   error
	}{
		"below minimum": {"4999", ErrBelowMinimum},
		"above balance": {"12001", ErrAboveBalance},
	} {
		if _, _, err := f.svc.Create(ctx, f.driver, key(), d(c.amount)); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", name, c.want, err)
		}
	}
	f.held = true
	if _, _, err := f.svc.Create(ctx, f.driver, key(), d("6000")); !errors.Is(err, ErrBlockedByHold) {
		t.Fatalf("a staff hold blocks cash-out: %v", err)
	}
	f.held = false
	k := key()
	p, replayed, err := f.svc.Create(ctx, f.driver, k, d("6000"))
	if err != nil || replayed || p.Status != StatusRequested {
		t.Fatalf("create: %+v %v %v", p, replayed, err)
	}
	if again, replayed, _ := f.svc.Create(ctx, f.driver, k, d("6000")); !replayed || again.ID != p.ID {
		t.Fatal("same key must replay")
	}
	if _, _, err := f.svc.Create(ctx, f.driver, key(), d("5000")); !errors.Is(err, ErrInProgress) {
		t.Fatalf("one open payout per driver: %v", err)
	}
	if _, err := f.svc.Cancel(ctx, f.driver, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.Create(ctx, f.driver, key(), d("5000")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.Create(ctx, f.driver, key(), d("5000")); !errors.Is(err, ErrInProgress) {
		t.Fatal(err)
	}
}

func TestStaffApprovalPaysAndPostsOnlyWhenPaid(t *testing.T) {
	f := newFixture(t, "9000", "0009")
	ctx := context.Background()
	p, _, err := f.svc.Create(ctx, f.driver, key(), d("7000"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Approve(ctx, f.staff, p.ID, "checked"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.send(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.send(ctx, p.ID); err != nil { // a retried job sends nothing
		t.Fatal(err)
	}
	if !f.owed(t).Equal(d("9000")) {
		t.Fatal("the ledger must not move before the gateway says PAID")
	}
	got, err := f.svc.Refresh(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = f.svc.Get(ctx, f.driver, got.ID)
	if got.Status != StatusPaid || !f.owed(t).Equal(d("2000")) {
		t.Fatalf("after paid: %s owed %s", got.Status, f.owed(t))
	}
	if _, err := f.svc.Refresh(ctx, p.ID); err != nil || !f.owed(t).Equal(d("2000")) {
		t.Fatal("a second check must not post again")
	}
}

func TestRejectAndAutomatic(t *testing.T) {
	f := newFixture(t, "20000", "0009")
	ctx := context.Background()
	p, _, _ := f.svc.Create(ctx, f.driver, key(), d("5000"))
	if _, err := f.svc.Reject(ctx, f.staff, p.ID, ""); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("a rejection needs a reason: %v", err)
	}
	r, err := f.svc.Reject(ctx, f.staff, p.ID, "wrong number on file")
	if err != nil || r.Status != StatusRejected {
		t.Fatalf("reject: %+v %v", r, err)
	}
	f.rules.AutoApprove = true
	auto, _, err := f.svc.Create(ctx, f.driver, key(), d("5000"))
	if err != nil || auto.Status != StatusPending || auto.ProviderRef == nil {
		t.Fatalf("automatic payouts go straight to the gateway: %+v %v", auto, err)
	}
}

func TestBalanceDropBeforeSendingFailsThePayout(t *testing.T) {
	f := newFixture(t, "8000", "0009")
	ctx := context.Background()
	p, _, _ := f.svc.Create(ctx, f.driver, key(), d("8000"))
	if _, err := f.svc.Approve(ctx, f.staff, p.ID, "ok"); err != nil {
		t.Fatal(err)
	}
	// A cash trip's platform share lands before the payout is sent.
	if _, err := f.ledger.Post(ctx, "test.debt", "debt-"+p.ID, "debt", "TZS", []money.Leg{
		{Kind: money.DriverWallet, OwnerID: f.driver, Amount: d("3000")}, {Kind: money.PlatformRevenue, Amount: d("-3000")}}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.send(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(ctx, f.driver, p.ID)
	if got.Status != StatusFailed || got.FailureCode == nil || *got.FailureCode != FailBalanceChanged || got.SentAt != nil {
		t.Fatalf("must fail without sending: %+v", got)
	}
}

func TestUnclearGatewayReplyStaysPendingUntilStatusAnswers(t *testing.T) {
	f := newFixture(t, "6000", "0004") // Disburse errors, but the money went
	ctx := context.Background()
	p, _, _ := f.svc.Create(ctx, f.driver, key(), d("6000"))
	_, _ = f.svc.Approve(ctx, f.staff, p.ID, "ok")
	if err := f.svc.send(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(ctx, f.driver, p.ID)
	if got.Status != StatusPending {
		t.Fatalf("an unclear reply must never fail a payout: %s", got.Status)
	}
	_, _ = f.svc.Refresh(ctx, p.ID)
	if got, _ = f.svc.Get(ctx, f.driver, p.ID); got.Status != StatusPaid {
		t.Fatalf("status says paid: %s", got.Status)
	}
}

func TestRefusedAndStuck(t *testing.T) {
	ctx := context.Background()
	refused := newFixture(t, "6000", "0002") // no wallet on the number
	p, _, _ := refused.svc.Create(ctx, refused.driver, key(), d("6000"))
	_, _ = refused.svc.Approve(ctx, refused.staff, p.ID, "ok")
	_ = refused.svc.send(ctx, p.ID)
	if got, _ := refused.svc.Get(ctx, refused.driver, p.ID); got.Status != StatusFailed || *got.FailureCode != "WALLET_INVALID" {
		t.Fatalf("refused: %+v", got)
	}
	stuck := newFixture(t, "6000", "0003") // never completes
	p, _, _ = stuck.svc.Create(ctx, stuck.driver, key(), d("6000"))
	_, _ = stuck.svc.Approve(ctx, stuck.staff, p.ID, "ok")
	_ = stuck.svc.send(ctx, p.ID)
	_, _ = stuck.pool.Exec(ctx, `UPDATE driver_payouts SET sent_at = now() - interval '31 minutes' WHERE id=$1`, p.ID)
	list, err := stuck.svc.AdminList(ctx, StatusPending, "TZS", 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list {
		if a.ID == p.ID && !a.Stuck {
			t.Fatal("a payout pending for 30 minutes must be flagged stuck")
		}
	}
}

func TestConcurrentApprovalsSendOnce(t *testing.T) {
	f := newFixture(t, "7000", "0009")
	ctx := context.Background()
	p, _, _ := f.svc.Create(ctx, f.driver, key(), d("7000"))
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.Approve(ctx, f.staff, p.ID, "ok"); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("exactly one approval must win, got %d", ok)
	}
}

func TestDriversCannotTouchEachOthersPayouts(t *testing.T) {
	a := newFixture(t, "9000", "0009")
	b := newFixture(t, "9000", "0009")
	ctx := context.Background()
	p, _, err := a.svc.Create(ctx, a.driver, key(), d("6000"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.svc.Cancel(ctx, b.driver, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another driver must not cancel it: %v", err)
	}
	if got, _ := a.svc.Get(ctx, a.driver, p.ID); got.Status != StatusRequested {
		t.Fatalf("still requested: %s", got.Status)
	}
	w, _ := a.svc.WalletOf(ctx, a.driver)
	if !w.Available.Equal(d("3000")) {
		t.Fatalf("the open payout is spoken for: available %s", w.Available)
	}
}

func TestRefusedPayoutPaidAfterAllIsPostedOnce(t *testing.T) {
	f := newFixture(t, "7000", "0001") // the gateway reports a refusal (float too low)
	ctx := context.Background()
	p, _, _ := f.svc.Create(ctx, f.driver, key(), d("6000"))
	_, _ = f.svc.Approve(ctx, f.staff, p.ID, "ok")
	_ = f.svc.send(ctx, p.ID)
	_, _ = f.svc.Refresh(ctx, p.ID)
	got, _ := f.svc.Get(ctx, f.driver, p.ID)
	if got.Status != StatusFailed || !f.owed(t).Equal(d("7000")) {
		t.Fatalf("refused: %s owed %s", got.Status, f.owed(t))
	}
	// The gateway later says it paid: the sweep posts it, once.
	for i := 0; i < 2; i++ {
		if err := f.svc.settleLate(ctx, got, "LATE1"); err != nil {
			t.Fatal(err)
		}
	}
	got, _ = f.svc.Get(ctx, f.driver, p.ID)
	if got.Status != StatusPaid || !f.owed(t).Equal(d("1000")) {
		t.Fatalf("late paid: %s owed %s", got.Status, f.owed(t))
	}
	_, _ = f.pool.Exec(ctx, `DELETE FROM audit_log WHERE target=$1`, "payout:"+p.ID)
}

func TestNoPayoutGatewayInACountry(t *testing.T) {
	f := newFixture(t, "9000", "0009")
	f.svc.gateways = payment.NewPayouters(map[string]payment.Payouter{"KES": f.gw}) // none for TZS
	ctx := context.Background()
	if _, _, err := f.svc.Create(ctx, f.driver, key(), d("6000")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no payout gateway in the driver's currency: %v", err)
	}
	if w, _ := f.svc.WalletOf(ctx, f.driver); w.Enabled {
		t.Fatal("the wallet says cash-out is off")
	}
}
