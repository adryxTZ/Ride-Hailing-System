package payments

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

// actionLateCollected is audited when money arrives after a pay-in expired.
const actionLateCollected = "payin.late_collected"

// Service owns pay-ins.
type Service struct {
	store       store
	gateways    *payment.Gateways
	debts       Debts
	ledger      Ledger
	holds       Holds
	limits      LimitSource
	jobs        JobEnqueuer
	notify      Notifier
	fares       FareTrips
	fareLedger  FareLedger
	country     string
	callbackURL string
	now         func() time.Time
}

// Deps is everything the service needs.
type Deps struct {
	Pool        *pgxpool.Pool
	BrandID     string
	Country     string            // for phone normalisation
	Gateways    *payment.Gateways // the collection gateway of each currency
	Debts       Debts
	Ledger      Ledger
	Holds       Holds
	Limits      LimitSource
	Jobs        JobEnqueuer
	Notify      Notifier
	Fares       FareTrips  // nil = rider fares are not collected through the gateway
	FareLedger  FareLedger // required with Fares
	CallbackURL string     // public webhook base, e.g. https://api/v1/payments/webhooks ("" = polling only); the gateway key is appended
}

// New builds the service.
func New(d Deps) *Service {
	return &Service{store: store{pool: d.Pool, brandID: d.BrandID}, gateways: d.Gateways, debts: d.Debts, ledger: d.Ledger,
		holds: d.Holds, limits: d.Limits, jobs: d.Jobs, notify: d.Notify, fares: d.Fares, fareLedger: d.FareLedger, country: d.Country, callbackURL: d.CallbackURL, now: time.Now}
}

// Create starts a debt payment: validates the amount against the debt and the
// minimum, stores a PENDING row (with the gateway reference and its check job)
// in one transaction, then pushes the USSD prompt. Replaying the same key
// returns the original (replayed=true).
func (s *Service) Create(ctx context.Context, driverID, key string, amount decimal.Decimal, rawPhone string) (*Payin, bool, error) {
	key = strings.TrimSpace(key)
	if len(key) < keyMinLen || len(key) > keyMaxLen {
		return nil, false, ErrKeyRequired
	}
	if existing, err := s.store.byKey(ctx, driverID, key); err != nil || existing != nil {
		return existing, existing != nil, err
	}
	phoneE164, err := phone.Normalize(rawPhone, s.country)
	if err != nil {
		return nil, false, apperr.Validation.WithDetails(map[string]any{"fields": []string{"phone"}})
	}
	currency, gw, err := s.validateAmount(ctx, driverID, amount)
	if err != nil {
		return nil, false, err
	}
	if err := s.checkPushLimits(ctx, driverID, phoneE164); err != nil {
		return nil, false, err
	}
	p := Payin{ID: uuid.NewString(), DriverID: driverID, Amount: amount, Currency: currency, Phone: phoneE164, Gateway: gw.Name(),
		ExpiresAt: s.now().Add(ExpireAfter), Purpose: purposeDriverDebt}
	req := s.collectRequest(p)
	ref := gw.Reference(req)
	p.ProviderRef = &ref
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if _, err := s.store.insert(ctx, tx, p, key); err != nil {
			return err
		}
		if s.jobs == nil {
			return nil
		}
		_, err := s.jobs.InsertTx(ctx, tx, CheckArgs{PayinID: p.ID}, &river.InsertOpts{ScheduledAt: s.now().Add(FirstCheck), UniqueOpts: river.UniqueOpts{ByArgs: true}})
		return err
	})
	if errors.Is(err, ErrInProgress) {
		if open, _ := s.store.pending(ctx, driverID); open != nil {
			return nil, false, ErrInProgress.WithDetails(map[string]any{"payin_id": open.ID})
		}
	}
	if err != nil {
		return nil, false, err
	}
	return s.push(ctx, p, req)
}

// checkPushLimits caps prompts per driver and per destination number: a prompt
// reaches a real phone, so this is both anti-harassment and anti-fraud.
func (s *Service) checkPushLimits(ctx context.Context, driverID, phoneE164 string) error {
	byDriver, byPhone, err := s.store.recentPushes(ctx, driverID, phoneE164)
	if err != nil {
		return err
	}
	if byDriver >= PushesPerDriverPerHour || byPhone >= PushesPerPhonePerDay {
		return ErrRateLimited
	}
	return nil
}

func (s *Service) collectRequest(p Payin) payment.CollectRequest {
	desc := "Debt payment"
	if p.Purpose == purposeRideFare {
		desc = "Trip fare"
	}
	return payment.CollectRequest{OrderID: p.ID, Phone: p.Phone, Amount: p.Amount, Currency: p.Currency, Description: desc, CallbackURL: s.callbackFor(p.Gateway)}
}

// callbackFor is the webhook address of one gateway ("" = polling only).
func (s *Service) callbackFor(gateway string) string {
	if s.callbackURL == "" {
		return ""
	}
	return s.callbackURL + "/" + gateway
}

// push sends the prompt. The row already carries the reference, so a reply lost
// in transit is still reconciled later; the push itself is not cancelled by the
// caller's request ending. A gateway error closes the row as GATEWAY_ERROR (the
// driver can retry at once) and the late re-check still credits money that arrives.
func (s *Service) push(ctx context.Context, p Payin, req payment.CollectRequest) (*Payin, bool, error) {
	gw, ok := s.gateways.Named(p.Gateway)
	if !ok {
		return nil, false, ErrGatewayDown
	}
	res, err := gw.Collect(context.WithoutCancel(ctx), req)
	if err != nil {
		slog.Warn("payments: gateway push failed", "payin", p.ID, "err", err)
		if _, ferr := s.store.finish(ctx, s.store.pool, p.ID, StatusPending, StatusFailed, payment.FailGateway); ferr != nil {
			return nil, false, ferr
		}
		s.fareFailed(ctx, &p, payment.FailGateway)
		return nil, false, ErrGatewayDown
	}
	if res.ProviderRef != "" && res.ProviderRef != *p.ProviderRef {
		if err := s.store.setProviderRef(ctx, p.ID, res.ProviderRef); err != nil {
			return nil, false, err
		}
	}
	out, err := s.store.get(ctx, p.ID, "")
	return out, false, err
}

// validateAmount checks the amount is whole and between the minimum and the debt.
func (s *Service) validateAmount(ctx context.Context, driverID string, amount decimal.Decimal) (string, payment.Gateway, error) {
	debt, currency, err := s.debts.DriverDebt(ctx, driverID)
	if err != nil {
		return "", nil, err
	}
	if !debt.IsPositive() {
		return "", nil, ErrNothingToPay
	}
	gw, ok := s.gateways.For(currency)
	if !ok {
		return "", nil, ErrGatewayDown // no mobile-money gateway in this driver's country yet
	}
	l, err := s.limits(ctx)
	if err != nil {
		return "", nil, err
	}
	min := decimal.Max(l.MinPayin, gw.MinAmount(currency))
	if !amount.Equal(amount.Floor()) || !amount.IsPositive() {
		return "", nil, apperr.Validation.WithDetails(map[string]any{"fields": []string{"amount"}})
	}
	if amount.LessThan(min) {
		return "", nil, ErrBelowMinimum.WithDetails(map[string]any{"min": min})
	}
	if max := debt.Ceil(); amount.GreaterThan(max) {
		return "", nil, ErrAboveDebt.WithDetails(map[string]any{"max": max})
	}
	return currency, gw, nil
}

// Refresh asks the gateway what happened and applies it. It is the only way a
// pay-in changes status. Returns the row afterwards (nil when unknown).
func (s *Service) Refresh(ctx context.Context, id string) (*Payin, error) {
	p, err := s.store.get(ctx, id, "")
	if err != nil || p == nil {
		return p, err
	}
	late := p.Status == StatusFailed && p.FailureCode != nil && recoverable(*p.FailureCode)
	if p.Status != StatusPending && !late {
		return p, nil
	}
	if p.ProviderRef == nil {
		return s.expireIfDue(ctx, p)
	}
	gw, ok := s.gateways.Named(p.Gateway)
	if !ok {
		slog.Warn("payments: gateway of a stored payment is not configured", "payin", id, "gateway", p.Gateway)
		return p, nil
	}
	st, err := gw.Status(ctx, *p.ProviderRef)
	if err != nil {
		slog.Warn("payments: status check failed", "payin", id, "err", err)
		return s.expireIfDue(ctx, p) // a gateway that stays down must not pin the driver to a pending row
	}
	if err := s.store.touch(ctx, id); err != nil {
		return nil, err
	}
	switch st.State {
	case payment.StateCollected:
		return s.collect(ctx, p, st.ProviderTxnID, late)
	case payment.StateFailed:
		if late {
			return p, nil
		}
		_, err := s.store.finish(ctx, s.store.pool, id, StatusPending, StatusFailed, st.FailureCode)
		if err != nil {
			return nil, err
		}
		s.fareFailed(ctx, p, st.FailureCode)
		s.tell(ctx, p)
		return s.store.get(ctx, id, "")
	}
	return s.expireIfDue(ctx, p)
}

// collect closes the row and posts the ledger movement in one transaction, then
// re-evaluates the driver's hold. A late success (after expiry) is audited.
// Fares settle their trip instead (collectFare).
func (s *Service) collect(ctx context.Context, p *Payin, providerTxn string, late bool) (*Payin, error) {
	if p.isFare() {
		return s.collectFare(ctx, p, late)
	}
	from := StatusPending
	if late {
		from = StatusFailed
	}
	moved := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.finish(ctx, tx, p.ID, from, StatusCollected, "")
		if err != nil || !ok {
			return err
		}
		moved = true
		ref := providerTxn
		if ref == "" && p.ProviderRef != nil {
			ref = *p.ProviderRef
		}
		if _, err := s.ledger.PostPayinCollected(ctx, tx, p.ID, p.DriverID, p.Amount, p.Currency, p.Gateway, ref); err != nil {
			return err
		}
		if late {
			return audit.Record(ctx, tx, s.store.brandID, "", actionLateCollected, "payin:"+p.ID, "", map[string]any{"amount": p.Amount, "driver": p.DriverID})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if moved {
		if s.holds != nil {
			if _, err := s.holds.Evaluate(ctx, p.DriverID); err != nil {
				slog.Warn("payments: hold evaluation after pay-in failed", "driver", p.DriverID, "err", err)
			}
		}
		s.tell(ctx, p)
	}
	return s.store.get(ctx, p.ID, "")
}

// expireIfDue fails a pending row whose prompt went unanswered.
func (s *Service) expireIfDue(ctx context.Context, p *Payin) (*Payin, error) {
	if p.Status != StatusPending || s.now().Before(p.ExpiresAt) {
		return p, nil
	}
	if _, err := s.store.finish(ctx, s.store.pool, p.ID, StatusPending, StatusFailed, FailExpired); err != nil {
		return nil, err
	}
	s.fareFailed(ctx, p, FailExpired)
	s.tell(ctx, p)
	return s.store.get(ctx, p.ID, "")
}

// Get returns a driver's own pay-in.
func (s *Service) Get(ctx context.Context, driverID, id string) (*Payin, error) {
	p, err := s.store.get(ctx, id, driverID)
	if err == nil && p == nil {
		return nil, ErrPayinNotFound
	}
	return p, err
}

// Pending returns the driver's open pay-in, or nil.
func (s *Service) Pending(ctx context.Context, driverID string) (*Payin, error) {
	return s.store.pending(ctx, driverID)
}

// List returns recent pay-ins for staff.
func (s *Service) List(ctx context.Context, status, driverID, currency, purpose string, limit int) ([]AdminPayin, error) {
	return s.store.list(ctx, status, driverID, currency, purpose, limit)
}

// Webhook handles a gateway callback: it only identifies the pay-in and queues
// an immediate re-check with the gateway; nothing in the payload decides anything.
// Unknown or settled ids cost one indexed read and no write, so a flood of
// made-up callbacks cannot fill the job table.
func (s *Service) Webhook(ctx context.Context, orderRef string) {
	if _, err := uuid.Parse(orderRef); err != nil {
		return // not one of ours; no work, no log line for attacker-chosen input
	}
	p, err := s.store.get(ctx, orderRef, "")
	if err != nil || p == nil || !p.stillOpen() {
		return
	}
	if s.jobs == nil {
		if _, err := s.Refresh(ctx, orderRef); err != nil {
			slog.Warn("payments: webhook refresh failed", "payin", orderRef, "err", err)
		}
		return
	}
	// Unique only against jobs that are about to run: the pay-in's own check job
	// may be snoozed, and a callback must not wait for its next tick.
	opts := &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable}}}
	if _, err := s.jobs.Insert(ctx, CheckArgs{PayinID: orderRef}, opts); err != nil {
		slog.Warn("payments: webhook enqueue failed", "payin", orderRef, "err", err)
	}
}

// stillOpen reports whether a gateway answer could still change this pay-in.
func (p *Payin) stillOpen() bool {
	return p.Status == StatusPending || (p.Status == StatusFailed && p.FailureCode != nil && recoverable(*p.FailureCode))
}

// recoverableFailures are failures after which the gateway may still report money.
var recoverableFailures = []string{FailExpired, payment.FailGateway}

func recoverable(code string) bool {
	for _, c := range recoverableFailures {
		if c == code {
			return true
		}
	}
	return false
}

// tell refreshes the driver's wallet in their app after a debt pay-in moved
// (fares tell both apps through the trip instead).
func (s *Service) tell(ctx context.Context, p *Payin) {
	if s.notify != nil && !p.isFare() {
		s.notify(ctx, p.DriverID)
	}
}

// KnowsGateway reports whether a gateway key (webhook path segment) is configured.
func (s *Service) KnowsGateway(name string) bool {
	_, ok := s.gateways.Named(name)
	return ok
}
