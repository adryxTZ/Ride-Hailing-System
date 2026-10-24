package approvals

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// ExpireEvery is how often pending requests past their time are closed. A
// decision on an overdue request also expires it on the spot.
const ExpireEvery = 5 * time.Minute

// ExpireDue closes every pending request past its expiry (audited per request).
func (s *Service) ExpireDue(ctx context.Context) error {
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ids, err := s.store.expireDue(ctx, tx, s.now())
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := audit.Record(ctx, tx, s.store.brandID, "", "approval.expired", "approval:"+id, "", nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// ExpireArgs is the periodic expiry job.
type ExpireArgs struct{}

// Kind names the job.
func (ExpireArgs) Kind() string { return "approvals.expire" }

// ExpireWorker runs ExpireDue; Svc is set once the service exists.
type ExpireWorker struct {
	river.WorkerDefaults[ExpireArgs]
	Svc *Service
}

// Work closes overdue requests.
func (w *ExpireWorker) Work(ctx context.Context, _ *river.Job[ExpireArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.ExpireDue(ctx)
}
