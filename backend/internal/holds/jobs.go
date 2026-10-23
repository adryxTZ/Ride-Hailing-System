package holds

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// Sweep cadence and page size. The sweep is what makes a soft-limit hold fire on
// time for a driver who stopped driving (no ledger movement would trigger it).
const (
	SweepInterval = 15 * time.Minute
	sweepBatch    = 500
)

// SweepArgs is the periodic sweep job.
type SweepArgs struct{}

// Kind names the job in River.
func (SweepArgs) Kind() string { return "holds.sweep" }

// SweepWorker re-evaluates every driver near or past a limit. Svc is set after
// the service is built (workers are registered before the River client exists).
type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	Svc *Service
}

// Work pages through the candidates and evaluates each one.
func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	if w.Svc == nil {
		return nil
	}
	_, err := w.Svc.Sweep(ctx)
	return err
}

// Sweep evaluates every candidate driver; it returns how many it looked at.
func (s *Service) Sweep(ctx context.Context) (int, error) {
	l, err := s.limits(ctx)
	if err != nil {
		return 0, err
	}
	n, after := 0, ""
	for {
		ids, err := s.store.sweepCandidates(ctx, l.Soft, after, sweepBatch)
		if err != nil || len(ids) == 0 {
			return n, err
		}
		for _, id := range ids {
			if _, err := s.Evaluate(ctx, id); err != nil {
				slog.Warn("holds: sweep evaluation failed", "driver", id, "err", err)
			}
		}
		n, after = n+len(ids), ids[len(ids)-1]
	}
}
