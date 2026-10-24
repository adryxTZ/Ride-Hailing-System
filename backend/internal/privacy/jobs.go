package privacy

import (
	"context"

	"github.com/riverqueue/river"
)

// ExportArgs builds one approved export.
type ExportArgs struct {
	RequestID string `json:"request_id"`
}

// Kind names the job in River.
func (ExportArgs) Kind() string { return "privacy.export" }

// RetentionArgs is the hourly check that runs retention once a day.
type RetentionArgs struct{}

// Kind names the job in River.
func (RetentionArgs) Kind() string { return "privacy.retention" }

// ExportWorker builds exports. Svc is set after the service is built.
type ExportWorker struct {
	river.WorkerDefaults[ExportArgs]
	Svc *Service
}

// Work builds the file.
func (w *ExportWorker) Work(ctx context.Context, j *river.Job[ExportArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.BuildExport(ctx, j.Args.RequestID)
}

// RetentionWorker runs the retention rules. Svc is set after the service is built.
type RetentionWorker struct {
	river.WorkerDefaults[RetentionArgs]
	Svc *Service
}

// Work runs the day's retention when it is due.
func (w *RetentionWorker) Work(ctx context.Context, _ *river.Job[RetentionArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.RunRetention(ctx)
}
