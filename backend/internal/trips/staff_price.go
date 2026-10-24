package trips

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// AdjustmentManual is the fare line for a price staff set by hand.
const AdjustmentManual = "MANUAL_PRICE"

// PricingManual is the pricing source of a trip whose fare staff set (no
// waiting fee is added on top of it later).
const PricingManual = "MANUAL"

// MaxManualPrice bounds a hand-set fare (numeric(14,2) holds less than 10^12).
var MaxManualPrice = decimal.New(1, 10)

// Set-price errors.
var (
	ErrPriceLocked       = apperr.New("TRIP_PRICE_LOCKED", http.StatusConflict, "")
	ErrPaymentInProgress = apperr.New("PAYMENT_IN_PROGRESS", http.StatusConflict, "")
	ErrPriceTooLow       = apperr.New("PRICE_BELOW_FIXED_PARTS", http.StatusConflict, "")
)

// AdminSetPrice replaces a trip's fare (without earlier cancellation dues) before
// it is paid, audited. The new fare is split at once under the trip's pinned
// terms, so a price below the booking fee and fixed charges is refused now
// rather than at the end of the ride; after the ride ended the stored split is
// replaced. A paid or settling trip is refunded instead.
func (s *Service) AdminSetPrice(ctx context.Context, staffID, id string, amount decimal.Decimal, reason string) (*Trip, error) {
	reason = strings.TrimSpace(reason)
	amount = amount.Round(2)
	if reason == "" || !amount.IsPositive() || amount.GreaterThan(MaxManualPrice) {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "amount_reason", "code": "REQUIRED"}}})
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if t == nil {
			return ErrTripNotFound
		}
		if err := priceChangeable(t); err != nil {
			return err
		}
		return s.setPriceTx(ctx, tx, staffID, t, amount, reason)
	})
	if err != nil {
		return nil, err
	}
	s.Notify(ctx, id)
	return s.store.get(ctx, s.store.pool, id, false)
}

// priceChangeable refuses ended trips and trips whose payment is under way or done.
func priceChangeable(t *Trip) error {
	if IsTerminal(t.Status) {
		return ErrPriceLocked
	}
	if t.PaymentStatus != nil {
		switch *t.PaymentStatus {
		case PayPending:
			return ErrPaymentInProgress
		case PayCollected, PayCashConfirmed:
			return ErrPriceLocked
		}
	}
	return nil
}

func (s *Service) setPriceTx(ctx context.Context, tx pgx.Tx, staffID string, t *Trip, amount decimal.Decimal, reason string) error {
	dues := t.DuesIn()
	before := fareAmount(t).Sub(dues)
	if err := s.checkSplit(ctx, tx, t, amount); err != nil {
		return err
	}
	adj := append(append([]Adjustment{}, t.Adjustments...), Adjustment{Code: AdjustmentManual, Amount: amount.Sub(before),
		Meta: map[string]any{"from": before.String(), "to": amount.String()}})
	raw, _ := json.Marshal(adj)
	// A failed mobile-money prompt is forgotten: a late success of it is then
	// treated as unapplied money owed back to the rider, never as this new fare.
	if _, err := tx.Exec(ctx, `UPDATE trips SET final_fare=$2, adjustments=$3, pricing_source=$4,
		payment_ref = CASE WHEN payment_status=$5 THEN NULL ELSE payment_ref END,
		payment_status = CASE WHEN payment_status=$5 THEN $6 ELSE payment_status END,
		updated_at=now(), version=version+1 WHERE id=$1`,
		t.ID, amount.Add(dues), raw, PricingManual, PayFailed, PayDue); err != nil {
		return err
	}
	if t.PaymentStatus != nil { // the ride ended: the stored split must follow the new fare
		fresh, err := s.store.get(ctx, tx, t.ID, false)
		if err != nil {
			return err
		}
		if err := s.fixSplitTx(ctx, tx, fresh); err != nil {
			return err
		}
	}
	if _, err := s.store.appendEvent(ctx, tx, t.ID, "trip.price_set", "STAFF", &staffID, map[string]any{"from": before, "to": amount, "reason": reason}); err != nil {
		return err
	}
	return audit.Record(ctx, tx, s.store.brandID, staffID, "trip.set_price", "trip:"+t.ID, reason, map[string]any{"from": before, "to": amount})
}

// checkSplit splits the new fare under the trip's terms without keeping the result.
func (s *Service) checkSplit(ctx context.Context, tx pgx.Tx, t *Trip, amount decimal.Decimal) error {
	if s.fees == nil {
		return nil
	}
	if _, err := s.fees(ctx, tx, t.ID, amount, bookingFeeOf(t.QuoteOption), t.Currency); err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return ErrPriceTooLow
	}
	return nil
}
