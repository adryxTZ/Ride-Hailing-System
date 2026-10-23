package trips

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Event types written when a fare's payment moves.
const (
	eventFarePrompted  = "payment.prompted"
	eventFareFailed    = "payment.failed"
	eventCashChosen    = "payment.cash_chosen"
	eventFareCollected = "payment.collected"
)

// FareFor returns the fare a rider may be prompted for now (their own trip,
// waiting for payment, no prompt open).
func (s *Service) FareFor(ctx context.Context, tripID, riderID string) (*FareDue, error) {
	t, err := s.store.get(ctx, s.store.pool, tripID, false)
	if err != nil {
		return nil, err
	}
	if t == nil || t.RiderID != riderID {
		return nil, ErrTripNotFound
	}
	if err := promptable(t); err != nil {
		return nil, err
	}
	return dueOf(t), nil
}

// dueOf is the collection request for a trip's fare.
func dueOf(t *Trip) *FareDue {
	return &FareDue{TripID: t.ID, RiderID: t.RiderID, Phone: t.Rider.Phone, Amount: fareAmount(t), Currency: t.Currency}
}

// FarePendingTx marks the fare as being collected by the gateway row payinID,
// inside the payments module's transaction (so a row and its trip agree).
func (s *Service) FarePendingTx(ctx context.Context, tx pgx.Tx, tripID, payinID string, auto bool) error {
	tag, err := tx.Exec(ctx, `UPDATE trips SET payment_status=$3, payment_ref=$2, cash_chosen=false
		WHERE id=$1 AND brand_id=$4 AND status='AWAITING_PAYMENT' AND payment_status IN ('DUE','FAILED')`,
		tripID, payinID, PayPending, s.store.brandID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPaymentNotDue
	}
	_, err = s.store.appendEvent(ctx, tx, tripID, eventFarePrompted, "SYSTEM", nil, map[string]any{"payin_id": payinID, "automatic": auto})
	return err
}

// FareFailed records that the prompt payinID did not go through; the rider may
// retry or pay cash. A stale failure (another prompt since) changes nothing.
func (s *Service) FareFailed(ctx context.Context, tripID, payinID, code string) error {
	moved := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE trips SET payment_status=$3 WHERE id=$1 AND payment_ref=$2 AND payment_status=$4 AND status='AWAITING_PAYMENT'`,
			tripID, payinID, PayFailed, PayPending)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		moved = true
		_, err = s.store.appendEvent(ctx, tx, tripID, eventFareFailed, "SYSTEM", nil, map[string]any{"payin_id": payinID, "code": code})
		return err
	})
	if err == nil && moved {
		s.Notify(ctx, tripID)
	}
	return err
}

// ChooseCash records that the rider will pay the driver in cash instead. The
// driver then confirms the cash; an open prompt must end first.
func (s *Service) ChooseCash(ctx context.Context, tripID, riderID string) (*Trip, error) {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.get(ctx, tx, tripID, true)
		if err != nil {
			return err
		}
		if t == nil || t.RiderID != riderID {
			return ErrTripNotFound
		}
		if err := promptable(t); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE trips SET cash_chosen=true WHERE id=$1`, tripID); err != nil {
			return err
		}
		_, err = s.store.appendEvent(ctx, tx, tripID, eventCashChosen, "RIDER", &riderID, map[string]any{})
		return err
	})
	if err != nil {
		return nil, err
	}
	s.Notify(ctx, tripID)
	return s.store.get(ctx, s.store.pool, tripID, false)
}

// CompletePaidTx completes a trip whose fare the gateway row payinID collected,
// inside the payments module's transaction. It returns the settlement to post,
// or nil when the trip was no longer waiting for that payment (the rider paid
// cash meanwhile, or another prompt is open): that money is then owed back.
func (s *Service) CompletePaidTx(ctx context.Context, tx pgx.Tx, tripID, payinID string) (*FareSettled, error) {
	t, err := s.store.get(ctx, tx, tripID, true)
	if err != nil || t == nil {
		return nil, err
	}
	if t.Status != AwaitingPayment || t.PaymentRef == nil || *t.PaymentRef != payinID || t.DriverID == nil {
		return nil, nil
	}
	if t.PaymentStatus == nil || (*t.PaymentStatus != PayPending && *t.PaymentStatus != PayFailed) {
		return nil, nil
	}
	if err := s.store.setStatus(ctx, tx, tripID, Completed, stampColumn[Completed], "payment_status = '"+PayCollected+"'"); err != nil {
		return nil, err
	}
	if _, err := s.store.appendEvent(ctx, tx, tripID, eventFareCollected, "SYSTEM", nil, map[string]any{"payin_id": payinID}); err != nil {
		return nil, err
	}
	if _, err := s.store.appendEvent(ctx, tx, tripID, "trip.status", "SYSTEM", nil, map[string]any{"status": Completed}); err != nil {
		return nil, err
	}
	done, err := s.store.get(ctx, tx, tripID, false)
	if err != nil {
		return nil, err
	}
	if err := s.completedTx(ctx, tx, done); err != nil {
		return nil, err
	}
	return &FareSettled{TripID: t.ID, RiderID: t.RiderID, DriverID: *t.DriverID, Fees: t.Fees, Dues: t.DuesIn(), Currency: t.Currency}, nil
}
