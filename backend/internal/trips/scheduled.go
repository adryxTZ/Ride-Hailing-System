package trips

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// scheduledBatch bounds how many booked-ahead rides one dispatcher tick starts.
const scheduledBatch = 100

// ActivateScheduled starts matching for rides booked ahead whose pickup is
// within ScheduleDispatchLead, and expires those whose matching window passed
// without starting (e.g. the rider was on another ride all along). It returns
// the ids that changed so the caller can notify the rider and dispatch.
func (s *Service) ActivateScheduled(ctx context.Context) ([]string, error) {
	due, err := s.dueScheduled(ctx)
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, id := range due {
		ok, err := s.activateOne(ctx, id)
		if err != nil {
			return changed, err
		}
		if ok {
			changed = append(changed, id)
		}
	}
	expired, err := s.expireScheduled(ctx)
	return append(changed, expired...), err
}

func (s *Service) dueScheduled(ctx context.Context) ([]string, error) {
	rows, err := s.store.pool.Query(ctx, `
		SELECT id FROM trips WHERE brand_id=$1 AND status='SCHEDULED' AND scheduled_for <= $2
		  AND dispatch_window_ends_at > $3 ORDER BY scheduled_for LIMIT $4`,
		s.store.brandID, s.now().Add(ScheduleDispatchLead), s.now(), scheduledBatch)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// activateOne moves one ride to REQUESTED. The one-active-ride-per-rider index
// may refuse it while the rider is still on another ride: it is retried next tick.
func (s *Service) activateOne(ctx context.Context, id string) (bool, error) {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE trips SET status='REQUESTED', requested_at=now(), updated_at=now(), version=version+1,
			  dispatch_window_ends_at = GREATEST(dispatch_window_ends_at, now() + $2::interval)
			WHERE id=$1 AND status='SCHEDULED'`, id, DispatchWindow.String())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = s.store.appendEvent(ctx, tx, id, "trip.requested", "SYSTEM", nil, map[string]any{"status": Requested, "from": Scheduled})
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) expireScheduled(ctx context.Context) ([]string, error) {
	var ids []string
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE trips SET status='EXPIRED', cancelled_at=now(), updated_at=now(), version=version+1
			WHERE brand_id=$1 AND status='SCHEDULED' AND dispatch_window_ends_at <= now() RETURNING id`, s.store.brandID)
		if err != nil {
			return err
		}
		if ids, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.store.appendEvent(ctx, tx, id, "trip.expired", "SYSTEM", nil, map[string]any{"status": Expired}); err != nil {
				return err
			}
		}
		return nil
	})
	return ids, err
}
