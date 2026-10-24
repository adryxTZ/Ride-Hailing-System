package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/privacy"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/storage"
	"github.com/kalen-limited/hauler-rider/backend/internal/refunds"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// privacyWorkers are registered before the River client exists.
type privacyWorkers struct {
	export    *privacy.ExportWorker
	retention *privacy.RetentionWorker
}

// registerPrivacyWorkers adds the export builder and the hourly retention check.
func registerPrivacyWorkers(workers *river.Workers) (privacyWorkers, []*river.PeriodicJob) {
	pw := privacyWorkers{export: &privacy.ExportWorker{}, retention: &privacy.RetentionWorker{}}
	river.AddWorker(workers, pw.export)
	river.AddWorker(workers, pw.retention)
	return pw, []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(privacy.RetentionCheckEvery), func() (river.JobArgs, *river.InsertOpts) {
			return privacy.RetentionArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "privacy.retention", RunOnStart: true}),
	}
}

// startPrivacy wires consent, privacy requests and retention. A retention rule
// the brand file names but the Platform does not know stops the start.
func startPrivacy(ctx context.Context, bs *bootstrap, brandID string, pw privacyWorkers, jobs *river.Client[pgx.Tx],
	files storage.Store, ledger *money.Ledger, p *presence.Service) (*privacy.Service, error) {
	for rule := range bs.brand.Privacy.Retention {
		if _, ok := privacy.RuleNames()[rule]; !ok {
			return nil, fmt.Errorf("brand %s: unknown retention rule %q", bs.brand.Code, rule)
		}
	}
	d := privacy.Deps{Pool: bs.pool, BrandID: brandID, Brand: bs.brand, Redis: bs.redis, Files: files, Money: ledger,
		Notify: smsSender(bs).Send, Location: countryLocation(bs, bs.brand.Country),
		EnqueueExportTx: func(ctx context.Context, tx pgx.Tx, id string) error {
			_, err := jobs.InsertTx(ctx, tx, privacy.ExportArgs{RequestID: id}, nil)
			return err
		}}
	if w, ok := files.(storage.Writer); ok {
		d.Writer = w
	}
	if p != nil {
		d.Offline = p.Offline
	}
	svc := privacy.New(d)
	if err := svc.SyncDocuments(ctx); err != nil {
		return nil, fmt.Errorf("consent documents: %w", err)
	}
	pw.export.Svc, pw.retention.Svc = svc, svc
	return svc, nil
}

// newRefunds builds staff refunds: posted to the ledger in the refund's
// transaction; the driver's wallet listeners and the trip's watchers hear after.
func newRefunds(bs *bootstrap, brandID string, ledger *money.Ledger, t *trips.Service) *refunds.Service {
	return refunds.New(bs.pool, brandID, ledger, func(ctx context.Context, tripID, driverID, bearer string) {
		if bearer == refunds.BorneByDriver && driverID != "" {
			ledger.WalletMoved(ctx, driverID)
		}
		t.Notify(ctx, tripID)
	})
}
