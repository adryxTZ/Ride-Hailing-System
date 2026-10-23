package payouts

import (
	"context"

	"github.com/riverqueue/river"
)

// SendArgs hands one payout to the gateway.
type SendArgs struct {
	PayoutID string `json:"payout_id"`
}

// Kind names the job in River.
func (SendArgs) Kind() string { return "payouts.send" }

// SendWorker sends a payout. Svc is set after the service is built.
type SendWorker struct {
	river.WorkerDefaults[SendArgs]
	Svc *Service
}

// Work sends the payout (a no-op when it was already sent).
func (w *SendWorker) Work(ctx context.Context, job *river.Job[SendArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.send(ctx, job.Args.PayoutID)
}

// CheckArgs re-checks one sent payout with the gateway.
type CheckArgs struct {
	PayoutID string `json:"payout_id"`
}

// Kind names the job in River.
func (CheckArgs) Kind() string { return "payouts.check" }

// CheckWorker polls a payout until it ends or is stuck (the sweep keeps checking stuck ones).
type CheckWorker struct {
	river.WorkerDefaults[CheckArgs]
	Svc *Service
}

// Work refreshes the payout and snoozes while it is pending and not yet stuck.
func (w *CheckWorker) Work(ctx context.Context, job *river.Job[CheckArgs]) error {
	if w.Svc == nil {
		return nil
	}
	p, err := w.Svc.Refresh(ctx, job.Args.PayoutID)
	if err != nil {
		return err
	}
	if p != nil && p.Status == StatusPending && p.SentAt != nil && w.Svc.now().Sub(*p.SentAt) < StuckAfter {
		return river.JobSnooze(CheckEvery)
	}
	return nil
}

// ReconcileArgs is the periodic sweep.
type ReconcileArgs struct{}

// Kind names the job in River.
func (ReconcileArgs) Kind() string { return "payouts.reconcile" }

// ReconcileWorker runs the sweep.
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
