package compliance

import (
	"context"

	"github.com/riverqueue/river"
)

// SweepArgs is the periodic send pass.
type SweepArgs struct{}

// Kind names the job in River.
func (SweepArgs) Kind() string { return "compliance.regulator.sweep" }

// SweepWorker runs the sweeper. Svc is set after the service is built.
type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	Svc *Service
}

// Work runs one pass.
func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.Sweep(ctx)
}

// DailyArgs queues the daily reports that are due.
type DailyArgs struct{}

// Kind names the job in River.
func (DailyArgs) Kind() string { return "compliance.regulator.daily" }

// DailyWorker queues daily reports. Svc is set after the service is built.
type DailyWorker struct {
	river.WorkerDefaults[DailyArgs]
	Svc *Service
}

// Work queues the reports that are due.
func (w *DailyWorker) Work(ctx context.Context, _ *river.Job[DailyArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.DailyReports(ctx)
}

// DocumentsArgs is the hourly document expiry pass.
type DocumentsArgs struct{}

// Kind names the job in River.
func (DocumentsArgs) Kind() string { return "compliance.documents.sweep" }

// DocumentsWorker runs the document expiry pass. Svc is set after the service is built.
type DocumentsWorker struct {
	river.WorkerDefaults[DocumentsArgs]
	Svc *Service
}

// Work runs one pass.
func (w *DocumentsWorker) Work(ctx context.Context, _ *river.Job[DocumentsArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.SweepDocuments(ctx)
}
