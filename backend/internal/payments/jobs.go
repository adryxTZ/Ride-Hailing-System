package payments

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
)

// CheckArgs re-checks one pay-in with the gateway.
type CheckArgs struct {
	PayinID string `json:"payin_id"`
}

// Kind names the job in River.
func (CheckArgs) Kind() string { return "payments.check" }

// CheckWorker polls one pay-in until it ends. Svc is set after the service is
// built (workers are registered before the River client exists).
type CheckWorker struct {
	river.WorkerDefaults[CheckArgs]
	Svc *Service
}

// Work refreshes the pay-in and snoozes while it is still open.
func (w *CheckWorker) Work(ctx context.Context, job *river.Job[CheckArgs]) error {
	if w.Svc == nil {
		return nil
	}
	p, err := w.Svc.Refresh(ctx, job.Args.PayinID)
	if err != nil {
		return err
	}
	if p != nil && p.Status == StatusPending {
		return river.JobSnooze(CheckEvery)
	}
	return nil
}

// ReconcileArgs is the periodic sweep.
type ReconcileArgs struct{}

// Kind names the job in River.
func (ReconcileArgs) Kind() string { return "payments.reconcile" }

// ReconcileWorker re-checks pending rows nobody checked lately and recent
// expired or gateway-error ones (a late gateway success is still credited).
type ReconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	Svc *Service
}

// Work runs one reconciliation pass.
func (w *ReconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.Reconcile(ctx)
}

// Reconcile refreshes stale pending and recently expired pay-ins.
func (s *Service) Reconcile(ctx context.Context) error {
	stale, err := s.store.stalePending(ctx, ReconcileStaleAfter, reconcileBatch)
	if err != nil {
		return err
	}
	expired, err := s.store.recentlyUnsettled(ctx, LateSuccessWindow, ReconcileStaleAfter, reconcileBatch)
	if err != nil {
		return err
	}
	for _, id := range append(stale, expired...) {
		if _, err := s.Refresh(ctx, id); err != nil {
			slog.Warn("payments: reconcile refresh failed", "payin", id, "err", err)
		}
	}
	return nil
}
