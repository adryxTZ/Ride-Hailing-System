package payments

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
)

// actionFareUnapplied is audited when fare money arrives for a trip that no longer waited for it.
const actionFareUnapplied = "fare.unapplied"

// FailSettlement marks a fare whose money arrived but could not settle the trip.
const FailSettlement = "SETTLEMENT_ERROR"

// Fare limits: a prompt reaches a real phone, so prompts are capped per trip and per number.
const (
	MaxFarePrompts       = 3
	FarePushesPerPhoneHr = 10
)

// Fare errors.
var (
	// ErrKeyReused: the idempotency key already belongs to another payment.
	ErrKeyReused       = apperr.New("IDEMPOTENCY_KEY_REUSED", 409, "")
	ErrFarePromptLimit = apperr.New("FARE_PROMPT_LIMIT", 429, "")
	// ErrBelowGatewayMinimum: the fare is smaller than the gateway accepts; the rider pays cash.
	ErrBelowGatewayMinimum = apperr.New("FARE_BELOW_GATEWAY_MINIMUM", 409, "")
)

// Fare is a trip fare to collect from its rider.
type Fare struct {
	TripID   string
	RiderID  string
	Phone    string // the rider's account phone; a prompt may go to another number
	Amount   decimal.Decimal
	Currency string
}

// FareState is the latest prompt of a trip, as the rider and driver apps show it.
type FareState struct {
	Status      string          `json:"status"` // PENDING, COLLECTED, FAILED
	FailureCode *string         `json:"failure_code,omitempty"`
	Phone       string          `json:"phone"`
	Amount      decimal.Decimal `json:"amount"`
	ExpiresAt   string          `json:"expires_at"`
	PromptsUsed int             `json:"prompts_used"`
	PromptsLeft int             `json:"prompts_left"`
}

// PromptFare sends a USSD prompt for a trip's fare: the row, its check job and
// the trip's PENDING state commit together, then the prompt is pushed. A replayed
// idempotency key returns the original row. auto marks the prompt sent when the
// ride ended (no key from the rider).
func (s *Service) PromptFare(ctx context.Context, f Fare, rawPhone, key string, auto bool) (*Payin, error) {
	if existing, err := s.ReplayFare(ctx, f.RiderID, f.TripID, key); err != nil || existing != nil {
		return existing, err
	}
	number := f.Phone
	if strings.TrimSpace(rawPhone) != "" {
		number = rawPhone
	}
	phoneE164, err := phone.Normalize(number, s.country)
	if err != nil {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []string{"phone"}})
	}
	if err := s.checkFareLimits(ctx, f, phoneE164); err != nil {
		return nil, err
	}
	gw, _ := s.gateways.For(f.Currency) // checked in checkFareLimits
	p := Payin{ID: uuid.NewString(), DriverID: f.RiderID, Amount: f.Amount, Currency: f.Currency, Phone: phoneE164,
		Gateway: gw.Name(), ExpiresAt: s.now().Add(ExpireAfter), Purpose: purposeRideFare, TripID: &f.TripID}
	req := s.collectRequest(p)
	ref := gw.Reference(req)
	p.ProviderRef = &ref
	if key == "" {
		key = "fare:" + p.ID
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if _, err := s.store.insert(ctx, tx, p, key); err != nil {
			return err
		}
		if err := s.fares.FarePendingTx(ctx, tx, f.TripID, p.ID, auto); err != nil {
			return err
		}
		return s.enqueueCheck(ctx, tx, p.ID)
	})
	if err != nil {
		return nil, err
	}
	s.fares.Notify(ctx, f.TripID)
	out, _, err := s.push(ctx, p, req)
	return out, err
}

// ReplayFare returns the prompt a rider already sent for this trip with this
// idempotency key, or nil (no key = no replay). A key used for another trip or
// another purpose is refused instead of answering with that other payment.
func (s *Service) ReplayFare(ctx context.Context, riderID, tripID, key string) (*Payin, error) {
	if key == "" {
		return nil, nil
	}
	if len(key) < keyMinLen || len(key) > keyMaxLen {
		return nil, ErrKeyRequired
	}
	p, err := s.store.byKey(ctx, riderID, key)
	if err != nil || p == nil {
		return nil, err
	}
	if !p.isFare() || *p.TripID != tripID {
		return nil, ErrKeyReused
	}
	return p, nil
}

// checkFareLimits applies the gateway minimum and the prompt caps.
func (s *Service) checkFareLimits(ctx context.Context, f Fare, phoneE164 string) error {
	gw, ok := s.gateways.For(f.Currency)
	if !ok {
		return ErrGatewayDown // no mobile-money gateway in this country: the rider pays cash
	}
	if min := gw.MinAmount(f.Currency); f.Amount.LessThan(min) {
		return ErrBelowGatewayMinimum.WithDetails(map[string]any{"min": min})
	}
	rows, err := s.store.fareRows(ctx, f.TripID)
	if err != nil {
		return err
	}
	if len(rows) >= MaxFarePrompts {
		return ErrFarePromptLimit
	}
	n, err := s.store.farePushesToPhone(ctx, phoneE164)
	if err != nil {
		return err
	}
	if n >= FarePushesPerPhoneHr {
		return ErrRateLimited
	}
	return nil
}

// enqueueCheck schedules the first status check of a new row in the caller's transaction.
func (s *Service) enqueueCheck(ctx context.Context, tx pgx.Tx, id string) error {
	if s.jobs == nil {
		return nil
	}
	_, err := s.jobs.InsertTx(ctx, tx, CheckArgs{PayinID: id}, &river.InsertOpts{ScheduledAt: s.now().Add(FirstCheck), UniqueOpts: river.UniqueOpts{ByArgs: true}})
	return err
}

// collectFare closes a fare row and settles its trip in one transaction: the
// trip completes (with its receipt) and the digital split is posted. Money for a
// trip that no longer waited for it is owed back to the rider, audited.
func (s *Service) collectFare(ctx context.Context, p *Payin, late bool) (*Payin, error) {
	from := StatusPending
	if late {
		from = StatusFailed
	}
	var settled *Settlement
	moved := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.finish(ctx, tx, p.ID, from, StatusCollected, "")
		if err != nil || !ok {
			return err
		}
		moved = true
		settled, err = s.fares.CompletePaidTx(ctx, tx, *p.TripID, p.ID)
		if err != nil {
			return err
		}
		if settled == nil {
			return s.unapplied(ctx, tx, p)
		}
		_, err = s.fareLedger.PostTripPaidTx(ctx, tx, *settled, p.Amount)
		return err
	})
	if err != nil && !transient(err) {
		return s.settlementFailed(ctx, p, from, err)
	}
	if err != nil {
		return nil, err
	}
	if moved {
		s.fares.Notify(ctx, *p.TripID)
		if settled != nil {
			s.fareLedger.WalletMoved(ctx, settled.DriverID)
		}
	}
	return s.store.get(ctx, p.ID, "")
}

// settlementFailed handles money that arrived but could not settle its trip for
// a lasting reason (the split or receipt disagrees with the amount): instead of
// retrying forever, the money is recorded as owed back to the rider (audited,
// logged as an error for Finance) and the trip is freed so the driver can take cash.
func (s *Service) settlementFailed(ctx context.Context, p *Payin, from string, cause error) (*Payin, error) {
	slog.Error("payments: collected fare could not settle its trip; owed back to the rider", "payin", p.ID, "trip", *p.TripID, "err", cause)
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.finish(ctx, tx, p.ID, from, StatusCollected, "")
		if err != nil || !ok {
			return err
		}
		if _, err := s.fareLedger.PostFareUnappliedTx(ctx, tx, p.ID, p.DriverID, p.Amount, p.Currency); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, "", actionFareUnapplied, "trip:"+*p.TripID, "settlement failed",
			map[string]any{"payin": p.ID, "amount": p.Amount, "rider": p.DriverID, "error": cause.Error()})
	})
	if err != nil {
		return nil, err
	}
	if ferr := s.fares.FareFailed(ctx, *p.TripID, p.ID, FailSettlement); ferr != nil {
		slog.Warn("payments: trip not freed after a settlement failure", "trip", *p.TripID, "err", ferr)
	}
	return s.store.get(ctx, p.ID, "")
}

// transient reports whether an error may pass on retry (connection, deadlock,
// serialization, cancelled request) rather than being a lasting refusal.
func transient(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		class := pgErr.Code[:2]
		return class == "08" || class == "40" || class == "53" || class == "57"
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// unapplied posts fare money that found no trip to pay as owed to the rider.
func (s *Service) unapplied(ctx context.Context, tx pgx.Tx, p *Payin) error {
	if _, err := s.fareLedger.PostFareUnappliedTx(ctx, tx, p.ID, p.DriverID, p.Amount, p.Currency); err != nil {
		return err
	}
	return audit.Record(ctx, tx, s.store.brandID, "", actionFareUnapplied, "trip:"+*p.TripID, "",
		map[string]any{"payin": p.ID, "amount": p.Amount, "rider": p.DriverID})
}

// fareFailed tells the trip that its prompt did not go through.
func (s *Service) fareFailed(ctx context.Context, p *Payin, code string) {
	if !p.isFare() || s.fares == nil {
		return
	}
	if err := s.fares.FareFailed(ctx, *p.TripID, p.ID, code); err != nil {
		slog.Warn("payments: fare failure not recorded on trip", "trip", *p.TripID, "err", err)
	}
}

// FareStateOf is the latest prompt of a trip (nil when none was sent).
func (s *Service) FareStateOf(ctx context.Context, tripID string) (*FareState, error) {
	rows, err := s.store.fareRows(ctx, tripID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	last := rows[0]
	return &FareState{Status: last.Status, FailureCode: last.FailureCode, Phone: last.Phone, Amount: last.Amount,
		ExpiresAt: last.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"), PromptsUsed: len(rows), PromptsLeft: max(0, MaxFarePrompts-len(rows))}, nil
}

// AutoPrompt sends the prompt when a mobile-money ride ends, in the background:
// the driver's "end ride" must not wait for the gateway. A fare under the
// gateway minimum falls back to cash (recorded on the trip); any other failure
// leaves the fare due, and the rider can retry or pay cash.
func (s *Service) AutoPrompt(ctx context.Context, f Fare, cashFallback func(ctx context.Context)) {
	if _, ok := s.gateways.For(f.Currency); !ok {
		cashFallback(ctx) // no mobile-money gateway in this country yet
		return
	}
	go func() {
		ctx := context.WithoutCancel(ctx)
		_, err := s.PromptFare(ctx, f, "", "", true)
		switch {
		case errors.Is(err, ErrBelowGatewayMinimum):
			cashFallback(ctx)
		case err != nil:
			slog.Warn("payments: automatic fare prompt failed; the rider can retry or pay cash", "trip", f.TripID, "err", err)
		}
	}()
}
