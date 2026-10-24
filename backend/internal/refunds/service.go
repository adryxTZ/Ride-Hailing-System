package refunds

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// Service is the refund use cases for one brand.
type Service struct {
	store  store
	ledger Ledger
	notify Notify
}

// New wires the service; notify may be nil.
func New(pool *pgxpool.Pool, brandID string, ledger Ledger, notify Notify) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, ledger: ledger, notify: notify}
}

// paidStatuses are the payment states after which money changed hands.
var paidStatuses = map[string]bool{"COLLECTED": true, "CASH_CONFIRMED": true}

// ForTrip is what was paid, refunded and is still refundable on a trip.
func (s *Service) ForTrip(ctx context.Context, tripID string) (*TripRefunds, error) {
	if _, err := uuid.Parse(tripID); err != nil {
		return nil, ErrTripNotFound
	}
	t, err := s.store.readTrip(ctx, nil, tripID, false) // a read never locks the trip
	if err != nil {
		return nil, err
	}
	out := &TripRefunds{Paid: t.paid, Refunded: t.refunded, Refundable: decimal.Max(decimal.Zero, t.paid.Sub(t.refunded)), Currency: t.currency}
	if !refundable(t) {
		out.Refundable = decimal.Zero
	}
	if out.Refunds, err = s.store.forTrip(ctx, tripID); err != nil {
		return nil, err
	}
	return out, nil
}

func refundable(t *paidTrip) bool {
	return t.status == "COMPLETED" && t.paymentStatus != nil && paidStatuses[*t.paymentStatus]
}

// Create refunds part or all of a completed, paid trip (audited); the ledger
// posting is in the same transaction. A repeated idempotency key returns the
// trip's refunds without refunding again (a retried request or double click).
func (s *Service) Create(ctx context.Context, staffID, tripID string, amount decimal.Decimal, bearer, reason, key string) (*TripRefunds, error) {
	reason, amount = strings.TrimSpace(reason), amount.Round(2)
	if reason == "" || len(reason) > reasonMaxLen || !amount.IsPositive() || (bearer != BorneByPlatform && bearer != BorneByDriver) {
		return nil, ErrInvalid
	}
	if _, err := uuid.Parse(tripID); err != nil {
		return nil, ErrTripNotFound
	}
	var driverID string
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.readTrip(ctx, tx, tripID, true)
		if err != nil {
			return err
		}
		if done, err := s.store.seen(ctx, tx, tripID, key); err != nil || done {
			return err
		}
		if !refundable(t) || (bearer == BorneByDriver && t.driverID == "") {
			return ErrNotRefundable
		}
		if amount.GreaterThan(t.paid.Sub(t.refunded)) {
			return ErrAmountTooLarge.WithDetails(map[string]any{"refundable": t.paid.Sub(t.refunded).String()})
		}
		driverID = t.driverID
		id, err := s.store.insert(ctx, tx, tripID, t, amount, bearer, reason, staffID, key)
		if err != nil {
			return err
		}
		if _, err := s.ledger.PostRefundTx(ctx, tx, id, t.riderID, t.driverID, bearer, amount, t.currency); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "trip.refund", "trip:"+tripID, reason,
			map[string]any{"refund": id, "amount": amount, "borne_by": bearer})
	})
	if err != nil {
		return nil, err
	}
	if s.notify != nil {
		s.notify(ctx, tripID, driverID, bearer)
	}
	return s.ForTrip(ctx, tripID)
}
