package trips

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Fare payment states (trips.payment_status). Money itself never moves here:
// the payments module collects it and calls back through FarePendingTx,
// FareFailed and CompletePaidTx; trips only records where the fare stands.
const (
	PayDue           = "DUE"            // the ride ended; nothing paid yet
	PayPending       = "PENDING"        // a mobile-money prompt is open
	PayFailed        = "FAILED"         // the last prompt did not go through
	PayCollected     = "COLLECTED"      // paid through the gateway
	PayCashConfirmed = "CASH_CONFIRMED" // the driver confirmed cash in hand
)

// Payment method values accepted at booking. Card and wallet fares wait for
// gateway credentials, so they are refused instead of completing unpaid.
const (
	MethodCash        = "CASH"
	MethodMobileMoney = "MOBILE_MONEY"
)

// Error codes owned by the fare payment rules.
var (
	ErrPaymentNotDue       = apperr.New("PAYMENT_NOT_DUE", http.StatusConflict, "")
	ErrFarePromptPending   = apperr.New("FARE_PROMPT_PENDING", http.StatusConflict, "")
	ErrMethodUnavailable   = apperr.New("PAYMENT_METHOD_UNAVAILABLE", http.StatusBadRequest, "")
	errFareSplitIncomplete = apperr.New("FARE_SPLIT_FAILED", http.StatusInternalServerError, "")
)

// TripPayment is where a finished ride's fare stands, as the apps show it.
// The API layer adds the open prompt (from the payments module) to it.
type TripPayment struct {
	Status   string          `json:"status"` // DUE, PENDING, FAILED, COLLECTED, CASH_CONFIRMED
	Method   string          `json:"method"`
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
	// CashChosen is true once the rider said they will pay cash instead.
	CashChosen bool `json:"cash_chosen"`
	// Prompt is the latest mobile-money prompt, when there is one.
	Prompt any `json:"prompt,omitempty"`
}

// FareDue is a fare ready to be collected: what the payments module needs to prompt the rider.
type FareDue struct {
	TripID   string
	RiderID  string
	Phone    string // the rider's account phone (default prompt number)
	Amount   decimal.Decimal
	Currency string
}

// FareSettled is a fare paid through the gateway, as the ledger posts it.
type FareSettled struct {
	TripID   string
	RiderID  string
	DriverID string
	Fees     []byte          // the fare split (money.Breakdown JSON) fixed at AWAITING_PAYMENT
	Dues     decimal.Decimal // earlier cancellation fees included in the amount
	Currency string
}

// CompletedTxHook runs inside the transaction that completes a trip (receipts are
// issued here, so a completed trip never exists without one). Nil = none.
type CompletedTxHook func(ctx context.Context, tx pgx.Tx, t *Trip) error

// FareDueHook runs after a ride reaches AWAITING_PAYMENT (the payments module
// sends the automatic prompt for mobile-money rides). Nil = none.
type FareDueHook func(ctx context.Context, t *Trip)

// PromptSource returns the latest mobile-money prompt of a trip (payments module).
type PromptSource func(ctx context.Context, tripID string) (any, error)

// SetPromptSource registers where open prompts are read from (nil = none shown).
func (s *Service) SetPromptSource(f PromptSource) { s.prompts = f }

// withPrompt adds the latest prompt to a trip whose fare is being paid, so both
// apps (API responses and live frames) can show the countdown and retries left.
func (s *Service) withPrompt(ctx context.Context, t *Trip) *Trip {
	if t == nil || t.Payment == nil || s.prompts == nil || t.Status != AwaitingPayment {
		return t
	}
	if p, err := s.prompts(ctx, t.ID); err == nil && p != nil {
		t.Payment.Prompt = p
	}
	return t
}

// OnCompletedTx adds a hook run inside the completing transaction, in
// registration order; any error aborts the completion.
func (s *Service) OnCompletedTx(h CompletedTxHook) { s.onCompletedTx = append(s.onCompletedTx, h) }

// completedTx runs the completion hooks for a just-completed trip.
func (s *Service) completedTx(ctx context.Context, tx pgx.Tx, t *Trip) error {
	for _, h := range s.onCompletedTx {
		if err := h(ctx, tx, t); err != nil {
			return err
		}
	}
	return nil
}

// OnFareDue registers what happens when a fare becomes due.
func (s *Service) OnFareDue(h FareDueHook) { s.onFareDue = h }

// fareAmount is what the rider pays at the end: the final fare, which already
// includes waiting charges and earlier cancellation dues.
func fareAmount(t *Trip) decimal.Decimal {
	if t.FinalFare != nil {
		return *t.FinalFare
	}
	return t.EstimatedFare
}

// cashConfirmable reports whether the driver may take cash now: never while a
// prompt is open (the money might still arrive), never twice.
func cashConfirmable(paymentStatus *string) error {
	if paymentStatus == nil {
		return nil // trips that reached AWAITING_PAYMENT before payment states existed
	}
	switch *paymentStatus {
	case PayDue, PayFailed:
		return nil
	case PayPending:
		return ErrFarePromptPending
	}
	return ErrInvalidTransition
}

// promptable reports whether a new prompt may be sent for this trip.
func promptable(t *Trip) error {
	if t.Status != AwaitingPayment || t.PaymentStatus == nil {
		return ErrPaymentNotDue
	}
	switch *t.PaymentStatus {
	case PayDue, PayFailed:
		return nil
	case PayPending:
		return ErrFarePromptPending
	}
	return ErrPaymentNotDue
}

// paymentOf is the read model shown to both apps once the ride has ended.
func paymentOf(t *Trip) *TripPayment {
	if t.PaymentStatus == nil {
		return nil
	}
	return &TripPayment{Status: *t.PaymentStatus, Method: t.PaymentMethod, Amount: fareAmount(t), Currency: t.Currency, CashChosen: t.cashChosen}
}

// fixFareTx freezes the fare and its split when the ride ends, so what the rider
// is asked to pay, what the receipt says and what the ledger posts are the same
// numbers. A split failure aborts the transition instead of leaving a ride with
// no breakdown.
func (s *Service) fixFareTx(ctx context.Context, tx pgx.Tx, t *Trip) error {
	if _, err := tx.Exec(ctx, `UPDATE trips SET final_fare = COALESCE(final_fare, estimated_fare), payment_status = $2 WHERE id=$1`, t.ID, PayDue); err != nil {
		return err
	}
	return s.fixSplitTx(ctx, tx, t)
}

// fixSplitTx stores the fare split; a failure aborts the transition, because a
// completed trip without a split would post nothing to the ledger.
func (s *Service) fixSplitTx(ctx context.Context, tx pgx.Tx, t *Trip) error {
	if s.fees == nil {
		return nil
	}
	fare := fareAmount(t).Sub(t.DuesIn())
	raw, err := s.fees(ctx, tx, t.ID, fare, bookingFeeOf(t.QuoteOption), t.Currency)
	if err != nil {
		slog.Error("fare split failed", "trip", t.ID, "err", err) // the cause stays in the logs, not the response
		return errFareSplitIncomplete
	}
	_, err = tx.Exec(ctx, `UPDATE trips SET fees=$2 WHERE id=$1`, t.ID, raw)
	return err
}
