package payouts

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

// Service owns driver payouts.
type Service struct {
	store    store
	gateways *payment.Payouters // the payout gateway of each currency
	ledger   Ledger
	rules    RuleSource
	blocked  Blocked
	person   Person
	jobs     JobEnqueuer
	notify   Notifier
	now      func() time.Time
}

// Deps is everything the service needs.
type Deps struct {
	Pool     *pgxpool.Pool
	BrandID  string
	Gateways *payment.Payouters // per currency; a currency without one has no payouts
	Ledger   Ledger
	Rules    RuleSource
	Blocked  Blocked
	Person   Person
	Jobs     JobEnqueuer
	Notify   Notifier
}

// New builds the service.
func New(d Deps) *Service {
	return &Service{store: store{pool: d.Pool, brandID: d.BrandID}, gateways: d.Gateways, ledger: d.Ledger, rules: d.Rules,
		blocked: d.Blocked, person: d.Person, jobs: d.Jobs, notify: d.Notify, now: time.Now}
}

// Enabled reports whether payouts can be sent in any currency.
func (s *Service) Enabled() bool { return s.gateways.Any() }

// gatewayFor is the payout gateway of a currency.
func (s *Service) gatewayFor(currency string) (payment.Payouter, bool) {
	if s.gateways == nil {
		return nil, false
	}
	return s.gateways.For(currency)
}

// gatewayOf is the payout gateway a stored payout uses.
func (s *Service) gatewayOf(p *Payout) (payment.Payouter, bool) {
	if s.gateways == nil {
		return nil, false
	}
	return s.gateways.Named(p.Gateway)
}

// Create asks for a cash-out to the driver's account phone. It waits for
// Finance, or goes straight to the gateway when the policy approves
// automatically. Replaying the same key returns the original (replayed=true).
func (s *Service) Create(ctx context.Context, driverID, key string, amount decimal.Decimal) (*Payout, bool, error) {
	if !s.Enabled() {
		return nil, false, ErrUnavailable
	}
	key = strings.TrimSpace(key)
	if len(key) < keyMinLen || len(key) > keyMaxLen {
		return nil, false, ErrKeyRequired
	}
	if existing, err := s.store.byKey(ctx, driverID, key); err != nil || existing != nil {
		return existing, existing != nil, err
	}
	rules, currency, err := s.check(ctx, driverID, amount)
	if err != nil {
		return nil, false, err
	}
	gw, _ := s.gatewayFor(currency) // checked in check
	phone, _, err := s.person(ctx, driverID)
	if err != nil {
		return nil, false, err
	}
	p := Payout{ID: uuid.NewString(), DriverID: driverID, Amount: amount, Currency: currency, Phone: phone, Gateway: gw.Name(), Status: StatusRequested}
	if rules.AutoApprove {
		p.Status = StatusPending
	}
	if err := s.insert(ctx, p, key); err != nil {
		return nil, false, err
	}
	s.tell(ctx, driverID)
	out, err := s.store.get(ctx, s.store.pool, p.ID, "", false)
	return out, false, err
}

// check applies every rule a request must pass and returns the rules and currency.
func (s *Service) check(ctx context.Context, driverID string, amount decimal.Decimal) (Rules, string, error) {
	rules, err := s.rules(ctx)
	if err != nil {
		return rules, "", err
	}
	owed, currency, err := s.ledger.DriverOwed(ctx, driverID)
	if err != nil {
		return rules, "", err
	}
	if !owed.IsPositive() {
		return rules, "", ErrNothingToPay
	}
	gw, ok := s.gatewayFor(currency)
	if !ok {
		return rules, "", ErrUnavailable // no payout gateway in this driver's country yet
	}
	if !amount.IsPositive() || !amount.Equal(amount.Floor()) {
		return rules, "", apperr.Validation.WithDetails(map[string]any{"fields": []string{"amount"}})
	}
	if min := decimal.Max(rules.Min, gw.MinPayout(currency)); amount.LessThan(min) {
		return rules, "", ErrBelowMinimum.WithDetails(map[string]any{"min": min})
	}
	if max := owed.Floor(); amount.GreaterThan(max) {
		return rules, "", ErrAboveBalance.WithDetails(map[string]any{"max": max})
	}
	if blocked, err := s.blocked(ctx, driverID); err != nil || blocked {
		return rules, "", firstErr(err, ErrBlockedByHold)
	}
	if n, err := s.store.requestsToday(ctx, driverID); err != nil || n >= MaxPerDay {
		return rules, "", firstErr(err, ErrRateLimited)
	}
	return rules, currency, nil
}

// insert stores the request; an automatic one gets its reference and send job in the same transaction.
func (s *Service) insert(ctx context.Context, p Payout, key string) error {
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if p.Status == StatusPending {
			gw, _ := s.gatewayOf(&p)
			ref := gw.PayoutReference(s.request(&p, ""))
			p.ProviderRef = &ref
		}
		if err := s.store.insert(ctx, tx, p, key); err != nil {
			return err
		}
		if p.Status != StatusPending {
			return nil
		}
		return s.enqueueSend(ctx, tx, p.ID)
	})
}

// Cancel withdraws a request Finance has not acted on yet.
func (s *Service) Cancel(ctx context.Context, driverID, id string) (*Payout, error) {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		p, err := s.store.get(ctx, tx, id, driverID, true) // only the driver's own payout
		if err != nil || p == nil {
			return firstErr(err, ErrNotFound)
		}
		ok, err := s.store.move(ctx, tx, id, StatusRequested, StatusCancelled, `, completed_at=now()`)
		if err == nil && !ok {
			return ErrNotActionable
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	s.tell(ctx, driverID)
	return s.Get(ctx, driverID, id)
}

// Get returns one of the driver's payouts.
func (s *Service) Get(ctx context.Context, driverID, id string) (*Payout, error) {
	p, err := s.store.get(ctx, s.store.pool, id, driverID, false)
	if err == nil && p == nil {
		return nil, ErrNotFound
	}
	return p, err
}

// List returns the driver's recent payouts, newest first.
func (s *Service) List(ctx context.Context, driverID string) ([]Payout, error) {
	return s.store.listForDriver(ctx, driverID, driverListLimit)
}

// WalletOf is the cash-out part of the driver's wallet: what they may cash out, the minimum, any open payout.
func (s *Service) WalletOf(ctx context.Context, driverID string) (Wallet, error) {
	w := Wallet{Available: decimal.Zero, Min: decimal.Zero}
	rules, err := s.rules(ctx)
	if err != nil {
		return w, err
	}
	owed, currency, err := s.ledger.DriverOwed(ctx, driverID)
	if err != nil {
		return w, err
	}
	w.Min = rules.Min
	if gw, ok := s.gatewayFor(currency); ok {
		w.Enabled, w.Min = true, decimal.Max(rules.Min, gw.MinPayout(currency))
	}
	if w.Open, err = s.store.openFor(ctx, driverID); err != nil {
		return w, err
	}
	w.Available = owed.Floor()
	if w.Open != nil { // the open payout is spoken for until it ends
		w.Available = decimal.Max(w.Available.Sub(w.Open.Amount), decimal.Zero)
	}
	return w, nil
}

// request is the gateway request for a payout.
func (s *Service) request(p *Payout, name string) payment.DisburseRequest {
	return payment.DisburseRequest{PayoutID: p.ID, Phone: p.Phone, RecipientName: name, Amount: p.Amount, Currency: p.Currency, Note: "Driver earnings"}
}

// enqueueSend schedules sending in the caller's transaction (one send job per payout).
func (s *Service) enqueueSend(ctx context.Context, tx pgx.Tx, id string) error {
	if s.jobs == nil {
		return nil
	}
	_, err := s.jobs.InsertTx(ctx, tx, SendArgs{PayoutID: id}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
	return err
}

func (s *Service) tell(ctx context.Context, driverID string) {
	if s.notify != nil {
		s.notify(ctx, driverID)
	}
}

func firstErr(err error, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
