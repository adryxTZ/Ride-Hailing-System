package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/kalen-limited/hauler-rider/backend/internal/fraud"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// fraudWorkers are registered before the River client exists.
type fraudWorkers struct {
	gps    *fraud.GPSFlagWorker
	trip   *fraud.TripCheckWorker
	cancel *fraud.CancelScanWorker
}

// registerFraudWorkers adds the GPS flag and trip check jobs and the hourly cancellation scan.
func registerFraudWorkers(workers *river.Workers) (fraudWorkers, []*river.PeriodicJob) {
	fw := fraudWorkers{gps: &fraud.GPSFlagWorker{}, trip: &fraud.TripCheckWorker{}, cancel: &fraud.CancelScanWorker{}}
	river.AddWorker(workers, fw.gps)
	river.AddWorker(workers, fw.trip)
	river.AddWorker(workers, fw.cancel)
	return fw, []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(fraud.CancelScanInterval), func() (river.JobArgs, *river.InsertOpts) {
			return fraud.CancelScanArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "fraud.cancel_scan", RunOnStart: true}),
	}
}

// startFraud wires the detectors: the GPS guard on every driver ping (Redis
// only), the self-booking check queued in the completing transaction, and the
// cancellation scan per country day.
func startFraud(bs *bootstrap, brandID string, fw fraudWorkers, jobs *river.Client[pgx.Tx], t *trips.Service, p *presence.Service) *fraud.Service {
	countries := make([]string, 0, len(bs.brand.Countries))
	for _, c := range bs.brand.Countries {
		countries = append(countries, c.Code)
	}
	svc := fraud.New(bs.pool, brandID, countries, func(country string) *time.Location { return countryLocation(bs, country) })
	fw.gps.Svc, fw.trip.Svc, fw.cancel.Svc = svc, svc, svc
	t.OnCompletedTx(func(ctx context.Context, tx pgx.Tx, trip *trips.Trip) error {
		if trip.DriverID == nil {
			return nil
		}
		_, err := jobs.InsertTx(ctx, tx, fraud.TripCheckArgs{TripID: trip.ID}, nil)
		return err
	})
	if p != nil && bs.redis != nil {
		guard := fraud.NewGPSGuard(bs.redis, bs.brand.Code, func(ctx context.Context, a fraud.GPSFlagArgs) error {
			_, err := jobs.Insert(ctx, a, nil)
			return err
		})
		p.OnPing(func(ctx context.Context, driverID string, prev *presence.Ping, cur presence.Ping) {
			guard.Observe(ctx, driverID, fraudPing(prev), *fraudPing(&cur))
		})
	}
	return svc
}

// fraudPing is a presence ping as the fraud check reads it.
func fraudPing(p *presence.Ping) *fraud.Ping {
	if p == nil {
		return nil
	}
	return &fraud.Ping{Lat: p.Location.Lat, Lng: p.Location.Lng, At: p.At, Mocked: p.Mocked}
}
