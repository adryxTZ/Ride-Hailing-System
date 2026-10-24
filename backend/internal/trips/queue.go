package trips

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Trip queue: a driver finishing a trip accepts the next one. The next trip is
// ACCEPTED with the driver set and queued_after pointing at the trip it waits
// for; it cannot start until that trip ends. Ending the trip ahead (completion
// or a staff cancel) promotes it, in the same transaction, to the driver's
// current trip. Dispatch decides who is offered what; trips keeps the rules.

// Error codes owned by the trip queue.
var (
	// ErrQueuedTripWaiting: the next trip starts only after the current one ends.
	ErrQueuedTripWaiting = apperr.New("NEXT_TRIP_WAITING", http.StatusConflict, "")
	// ErrQueueFull: the driver already has a next trip.
	ErrQueueFull = apperr.New("NEXT_TRIP_ALREADY_QUEUED", http.StatusConflict, "")
)

// Constraint names the queue relies on (00045_trip_queue.sql).
const (
	constraintOneQueued = "trips_one_queued_per_driver"
	pgUniqueViolation   = "23505"
)

// minQueuedEtaS is the floor of the pickup estimate shown while queued, so a
// promise that has passed never reads "0 min".
const minQueuedEtaS = 60

// Events written by the queue (trip_events.type).
const (
	eventQueuePromoted = "trip.queue_promoted"
	eventQueueReleased = "trip.queue_released"
)

// applyQueue fills the queue read model of a scanned trip: the flag, the
// pickup estimate, and no "start driving" button while the trip ahead runs.
func applyQueue(t *Trip, now time.Time) {
	if t.QueuedAfter == nil || t.Status != Accepted {
		return
	}
	t.Queued = true
	if t.queuedPickupBy != nil {
		eta := max(int(t.queuedPickupBy.Sub(now).Seconds()), minQueuedEtaS)
		t.QueuedPickupEtaS = &eta
	}
	next := make([]Status, 0, len(t.AllowedNext))
	for _, st := range t.AllowedNext {
		if st != EnRouteToPickup {
			next = append(next, st)
		}
	}
	t.AllowedNext = next
}

// freeCancelFrom is when the rider's free-cancel window starts: acceptance, or
// for a queued trip the moment it became current (the wait before is not theirs).
func freeCancelFrom(t *Trip) *time.Time {
	if t.queuePromotedAt != nil {
		return t.queuePromotedAt
	}
	return t.Timestamps["accepted_at"]
}

// AssignNext moves OFFERED → ACCEPTED like Assign. When the driver still has a
// current trip the new one is queued behind it, with the promised pickup time
// pickupEtaS from now; it reports whether the trip was queued. A driver whose
// trip ended meanwhile simply gets it as their current trip.
func (s *Service) AssignNext(ctx context.Context, tx pgx.Tx, tripID, driverID string, pickupEtaS int) (bool, error) {
	current, err := s.store.currentTripTx(ctx, tx, driverID, tripID)
	if err != nil {
		return false, err
	}
	if current == "" {
		return false, s.Assign(ctx, tx, tripID, driverID)
	}
	tag, err := tx.Exec(ctx, `UPDATE trips SET status='ACCEPTED', driver_id=$2, accepted_at=now(), queued_after=$3,
		queued_pickup_by = now() + make_interval(secs => $4), updated_at=now(), version=version+1
		WHERE id=$1 AND status='OFFERED'`, tripID, driverID, current, pickupEtaS)
	if isUnique(err, constraintOneQueued) {
		return false, ErrQueueFull
	}
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, ErrInvalidTransition
	}
	_, err = s.store.appendEvent(ctx, tx, tripID, "trip.accepted", "DRIVER", &driverID, map[string]any{"status": Accepted, "queued_after": current})
	return true, err
}

// QueuedForDriver returns the driver's next trip (accepted, waiting), or nil.
func (s *Service) QueuedForDriver(ctx context.Context, driverID string) (*Trip, error) {
	var id string
	err := s.store.pool.QueryRow(ctx, `SELECT id FROM trips WHERE driver_id=$1 AND brand_id=$2 AND status='ACCEPTED' AND queued_after IS NOT NULL`,
		driverID, s.store.brandID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.store.get(ctx, s.store.pool, id, false)
}

// promoteQueuedTx makes the trip queued behind endedID the driver's current
// trip. Called in the transaction that ends endedID; returns the promoted id ("" = none).
func (s *Service) promoteQueuedTx(ctx context.Context, tx pgx.Tx, endedID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `UPDATE trips SET queued_after=NULL, queue_promoted_at=now(), updated_at=now(), version=version+1
		WHERE queued_after=$1 AND status='ACCEPTED' RETURNING id`, endedID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	_, err = s.store.appendEvent(ctx, tx, id, eventQueuePromoted, "SYSTEM", nil, map[string]any{"after": endedID})
	return id, err
}

// queueGate refuses to start a queued trip while the trip ahead is still
// running. A trip ahead that ended without promoting this one (it cannot
// happen through the ending paths, which promote in their transaction) is
// promoted here instead of leaving the trip stuck.
func (s *Service) queueGate(ctx context.Context, tx pgx.Tx, t *Trip, to Status) error {
	if t.QueuedAfter == nil || t.Status != Accepted || to == CancelledByDriver {
		return nil
	}
	var ahead Status
	if err := tx.QueryRow(ctx, `SELECT status::text FROM trips WHERE id=$1`, *t.QueuedAfter).Scan(&ahead); err != nil {
		return err
	}
	if !IsTerminal(ahead) {
		return ErrQueuedTripWaiting
	}
	_, err := s.promoteQueuedTx(ctx, tx, *t.QueuedAfter)
	return err
}

// ReleaseQueued sends a driver's queued trip back to matching (the driver went
// offline before starting it). False when the trip is no longer that driver's
// queued trip.
func (s *Service) ReleaseQueued(ctx context.Context, tripID, driverID string) (bool, error) {
	released := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.get(ctx, tx, tripID, true)
		if err != nil || t == nil || !t.Queued || t.DriverID == nil || *t.DriverID != driverID {
			return err
		}
		if err := s.requeueTx(ctx, tx, tripID); err != nil {
			return err
		}
		released = true
		_, err = s.store.appendEvent(ctx, tx, tripID, eventQueueReleased, "SYSTEM", nil, map[string]any{"status": Requested, "driver_id": driverID})
		return err
	})
	return released, err
}

// requeueTx puts an assigned trip back into matching with a fresh window and
// no driver (a driver cancel before pickup, or a released queued trip).
func (s *Service) requeueTx(ctx context.Context, tx pgx.Tx, tripID string) error {
	_, err := tx.Exec(ctx, `UPDATE trips SET status='REQUESTED', driver_id=NULL, accepted_at=NULL, en_route_at=NULL, arrived_pickup_at=NULL,
		queued_after=NULL, queued_pickup_by=NULL, queue_promoted_at=NULL,
		redispatch_count = redispatch_count + 1, dispatch_window_ends_at = $2, updated_at=now(), version=version+1 WHERE id=$1`,
		tripID, s.now().Add(DispatchWindow))
	return err
}

// currentTripTx locks and returns the driver's current trip other than
// exceptID ("" = none), so a queued accept and a completion never race.
func (s store) currentTripTx(ctx context.Context, tx pgx.Tx, driverID, exceptID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM trips WHERE driver_id=$1 AND brand_id=$2 AND status = ANY($3::trip_status[])
		AND queued_after IS NULL AND id <> $4 FOR UPDATE`, driverID, s.brandID, statusStrings(ActiveStatuses), exceptID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// isUnique reports whether err is a unique violation of the named constraint.
func isUnique(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == constraint
}
