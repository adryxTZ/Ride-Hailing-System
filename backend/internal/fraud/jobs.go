package fraud

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

// CancelScanInterval is how often the cancellation scan checks whether a country's day is due.
const CancelScanInterval = time.Hour

// GPSFlagArgs raises a fake-GPS flag (queued by the ping-path guard).
type GPSFlagArgs struct {
	DriverID string     `json:"driver_id"`
	Mocked   bool       `json:"mocked"`
	Pairs    []PingPair `json:"pairs"`
	At       time.Time  `json:"at"`
}

// Kind names the job in River.
func (GPSFlagArgs) Kind() string { return "fraud.gps_flag" }

// TripCheckArgs checks a completed trip for self-booking.
type TripCheckArgs struct {
	TripID string `json:"trip_id"`
}

// Kind names the job in River.
func (TripCheckArgs) Kind() string { return "fraud.trip_check" }

// InsertOpts makes the check unique per trip.
func (TripCheckArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// CancelScanArgs runs the hourly cancellation scan check.
type CancelScanArgs struct{}

// Kind names the job in River.
func (CancelScanArgs) Kind() string { return "fraud.cancel_scan" }

// GPSFlagWorker raises GPS flags. Svc is set after the service is built.
type GPSFlagWorker struct {
	river.WorkerDefaults[GPSFlagArgs]
	Svc *Service
}

// Work raises the flag.
func (w *GPSFlagWorker) Work(ctx context.Context, j *river.Job[GPSFlagArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.GPSFlag(ctx, j.Args)
}

// TripCheckWorker checks completed trips. Svc is set after the service is built.
type TripCheckWorker struct {
	river.WorkerDefaults[TripCheckArgs]
	Svc *Service
}

// Work checks one trip.
func (w *TripCheckWorker) Work(ctx context.Context, j *river.Job[TripCheckArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.CheckTrip(ctx, j.Args.TripID)
}

// CancelScanWorker runs the cancellation scan. Svc is set after the service is built.
type CancelScanWorker struct {
	river.WorkerDefaults[CancelScanArgs]
	Svc *Service
}

// Work runs the scan for every country that is due.
func (w *CancelScanWorker) Work(ctx context.Context, _ *river.Job[CancelScanArgs]) error {
	if w.Svc == nil {
		return nil
	}
	return w.Svc.ScanCancellations(ctx)
}
