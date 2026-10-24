package trips

import (
	"context"
	"encoding/json"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Operations the dispatch module performs on trips. They take the dispatcher's
// transaction so an offer row and the trip status always change together.
// After its commit the dispatcher calls Notify so listeners (realtime) hear of it.

// OnChanged adds a listener told about every committed trip change (realtime,
// dispatch, ledger…). Listeners run in registration order after the commit.
func (s *Service) OnChanged(fn func(ctx context.Context, t *Trip)) {
	s.onChanged = append(s.onChanged, fn)
}

// Notify reloads a trip and passes it to the change listener (after commit).
func (s *Service) Notify(ctx context.Context, tripID string) {
	if len(s.onChanged) == 0 {
		return
	}
	if t, err := s.store.get(ctx, s.store.pool, tripID, false); err == nil && t != nil {
		s.withPrompt(ctx, t)
		for _, fn := range s.onChanged {
			fn(ctx, t)
		}
	}
}

// Load returns a trip without a participant check (internal modules only).
func (s *Service) Load(ctx context.Context, tripID string) (*Trip, error) {
	return s.store.get(ctx, s.store.pool, tripID, false)
}

// OpenRequests lists trips waiting for a driver (no open offer), oldest first.
func (s *Service) OpenRequests(ctx context.Context, limit int) ([]OpenRequest, error) {
	return s.store.openRequests(ctx, limit)
}

// surgeLookback counts recent unserved requests as demand too.
const surgeLookback = 10 * time.Minute

// demandLimit bounds the points one surge pass reads.
const demandLimit = 5000

// DemandPoints are the pickups of rides waiting for a driver now and of those
// that found none recently: the demand side of surge.
func (s *Service) DemandPoints(ctx context.Context) ([]routing.Point, error) {
	rows, err := s.store.pool.Query(ctx, `
		SELECT ST_Y(st.location::geometry), ST_X(st.location::geometry)
		FROM trips t JOIN trip_stops st ON st.trip_id = t.id AND st.kind = 'PICKUP'
		WHERE t.brand_id=$1 AND (t.status IN ('REQUESTED','OFFERED')
		  OR (t.status='NO_DRIVER_FOUND' AND t.cancelled_at > $2))
		LIMIT $3`, s.store.brandID, s.now().Add(-surgeLookback), demandLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []routing.Point
	for rows.Next() {
		var p routing.Point
		if err := rows.Scan(&p.Lat, &p.Lng); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkOffered moves REQUESTED → OFFERED. False when the trip moved on meanwhile.
func (s *Service) MarkOffered(ctx context.Context, tx pgx.Tx, tripID, driverID string) (bool, error) {
	tag, err := tx.Exec(ctx, `UPDATE trips SET status='OFFERED', offered_at=now(), updated_at=now(), version=version+1 WHERE id=$1 AND status='REQUESTED'`, tripID)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	_, err = s.store.appendEvent(ctx, tx, tripID, "trip.offered", "SYSTEM", nil, map[string]any{"driver_id": driverID})
	return err == nil, err
}

// Release moves OFFERED → REQUESTED after a decline or timeout, so matching continues.
func (s *Service) Release(ctx context.Context, tx pgx.Tx, tripID, reason string) error {
	tag, err := tx.Exec(ctx, `UPDATE trips SET status='REQUESTED', updated_at=now(), version=version+1 WHERE id=$1 AND status='OFFERED'`, tripID)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	_, err = s.store.appendEvent(ctx, tx, tripID, "trip.offer_"+reason, "SYSTEM", nil, nil)
	return err
}

// Assign moves OFFERED → ACCEPTED with the driver who accepted. Callers have
// already locked and validated the offer in the same transaction.
func (s *Service) Assign(ctx context.Context, tx pgx.Tx, tripID, driverID string) error {
	tag, err := tx.Exec(ctx, `UPDATE trips SET status='ACCEPTED', driver_id=$2, accepted_at=now(), updated_at=now(), version=version+1 WHERE id=$1 AND status='OFFERED'`, tripID, driverID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidTransition
	}
	_, err = s.store.appendEvent(ctx, tx, tripID, "trip.accepted", "DRIVER", &driverID, map[string]any{"status": Accepted})
	return err
}

// ExpireUnmatched ends trips whose matching window passed (NO_DRIVER_FOUND) and
// returns their ids. Open offers on them are closed by the caller.
func (s *Service) ExpireUnmatched(ctx context.Context) ([]string, error) {
	var ids []string
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE trips SET status='NO_DRIVER_FOUND', cancelled_at=now(), updated_at=now(), version=version+1
			WHERE brand_id=$1 AND status IN ('REQUESTED','OFFERED') AND dispatch_window_ends_at <= now() RETURNING id`, s.store.brandID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			if _, err := s.store.appendEvent(ctx, tx, id, "trip.no_driver_found", "SYSTEM", nil, map[string]any{"status": NoDriverFound}); err != nil {
				return err
			}
		}
		return nil
	})
	return ids, err
}

// ActiveForDriver returns the driver's assigned, unfinished trip, or nil.
func (s *Service) ActiveForDriver(ctx context.Context, driverID string) (*Trip, error) {
	t, err := s.store.activeForDriver(ctx, driverID)
	return s.withPrompt(ctx, t), err
}

// stampColumn maps a status to the timestamp column it sets.
var stampColumn = map[Status]string{
	EnRouteToPickup: "en_route_at", ArrivedAtPickup: "arrived_pickup_at",
	InTransit: "in_transit_at", ArrivedAtDropoff: "arrived_dropoff_at",
	AwaitingPayment: "awaiting_payment_at", Completed: "completed_at",
}

// arrivalStop maps an arrival status to the stop it marks as reached.
var arrivalStop = map[Status]string{ArrivedAtPickup: "PICKUP", ArrivedAtDropoff: "DROPOFF"}

// TransitionByDriver moves the driver's trip along its lifecycle. A driver
// cancel before the service starts sends the trip back to matching (another
// driver is found); the trip is never left without a driver silently.
func (s *Service) TransitionByDriver(ctx context.Context, tripID, driverID string, to Status, reason string) (*Trip, error) {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.get(ctx, tx, tripID, true)
		if err != nil {
			return err
		}
		if t == nil {
			return ErrTripNotFound
		}
		if t.DriverID == nil || *t.DriverID != driverID {
			return ErrNotAssigned
		}
		if err := s.queueGate(ctx, tx, t, to); err != nil {
			return err
		}
		if to == CancelledByRider || !CanTransition(t.Status, to, t.Lifecycle) {
			return ErrInvalidTransition
		}
		if to == CancelledByDriver && reason == ReasonNoShow {
			return s.noShow(ctx, tx, t, driverID)
		}
		if to == CancelledByDriver {
			return s.redispatch(ctx, tx, tripID, driverID)
		}
		if to == InTransit {
			if err := s.chargeWaiting(ctx, tx, t); err != nil {
				return err
			}
		}
		if err := s.beforeStatus(ctx, tx, t, to); err != nil {
			return err
		}
		extra := ""
		if to == Completed {
			extra = "final_fare = COALESCE(final_fare, estimated_fare), payment_status = '" + PayCashConfirmed + "'"
		}
		if err := s.store.setStatus(ctx, tx, tripID, to, stampColumn[to], extra); err != nil {
			return err
		}
		if to == Completed {
			if err := s.afterCompletedTx(ctx, tx, t); err != nil {
				return err
			}
		}
		if kind, ok := arrivalStop[to]; ok {
			if _, err := tx.Exec(ctx, `UPDATE trip_stops SET arrived_at=now() WHERE trip_id=$1 AND kind=$2 AND arrived_at IS NULL`, tripID, kind); err != nil {
				return err
			}
		}
		_, err = s.store.appendEvent(ctx, tx, tripID, "trip.status", "DRIVER", &driverID, map[string]any{"status": to})
		return err
	})
	if err != nil {
		return nil, err
	}
	s.Notify(ctx, tripID)
	out, err := s.store.get(ctx, s.store.pool, tripID, false)
	if err == nil && out != nil && to == AwaitingPayment && s.onFareDue != nil {
		s.onFareDue(ctx, out) // the prompt goes out in the background; the apps hear when it is PENDING
	}
	return s.withPrompt(ctx, out), err
}

// beforeStatus applies the fare rules of a driver transition: the fare is fixed
// when the ride ends, and the driver confirms cash only when no prompt is open.
func (s *Service) beforeStatus(ctx context.Context, tx pgx.Tx, t *Trip, to Status) error {
	switch to {
	case AwaitingPayment:
		return s.fixFareTx(ctx, tx, t)
	case Completed:
		return cashConfirmable(t.PaymentStatus)
	}
	return nil
}

// afterCompletedTx finishes a cash completion: the split exists (trips that ended
// before fares were fixed at AWAITING_PAYMENT get it now) and the receipt is issued.
func (s *Service) afterCompletedTx(ctx context.Context, tx pgx.Tx, t *Trip) error {
	if len(t.Fees) == 0 { // earlier dues are settled apart from the split
		if err := s.fixSplitTx(ctx, tx, t); err != nil {
			return err
		}
	}
	done, err := s.store.get(ctx, tx, t.ID, false)
	if err != nil {
		return err
	}
	return s.completedTx(ctx, tx, done)
}

// redispatch puts a driver-cancelled trip back into matching with a fresh window.
func (s *Service) redispatch(ctx context.Context, tx pgx.Tx, tripID, driverID string) error {
	if err := s.requeueTx(ctx, tx, tripID); err != nil {
		return err
	}
	_, err := s.store.appendEvent(ctx, tx, tripID, "trip.driver_cancelled", "DRIVER", &driverID, map[string]any{"status": Requested})
	return err
}

// chargeWaiting adds the pickup waiting fee (arrival → start) to the fare when
// the driver starts the ride.
func (s *Service) chargeWaiting(ctx context.Context, tx pgx.Tx, t *Trip) error {
	arrived := t.Timestamps["arrived_pickup_at"]
	if arrived == nil || t.PricingSource == PricingManual { // a price staff set is the whole fare
		return nil
	}
	fee, minutes := waitingFee(termsOf(t.QuoteOption), *arrived, s.now())
	if !fee.IsPositive() {
		return nil
	}
	lines := append(append([]Adjustment{}, t.Adjustments...), Adjustment{Code: AdjustmentWaiting, Amount: fee, Meta: map[string]any{"minutes": minutes}})
	adj, _ := json.Marshal(lines)
	_, err := tx.Exec(ctx, `UPDATE trips SET adjustments=$2, final_fare=COALESCE(final_fare, estimated_fare) + $3 WHERE id=$1`, t.ID, adj, fee)
	return err
}

// RecordEvent appends an event to a trip's log (e.g. an SOS) without changing its state.
func (s *Service) RecordEvent(ctx context.Context, tripID, typ, actorKind string, actorID *string, payload any) error {
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM trips WHERE id=$1 FOR UPDATE`, tripID); err != nil {
			return err
		}
		_, err := s.store.appendEvent(ctx, tx, tripID, typ, actorKind, actorID, payload)
		return err
	})
}
