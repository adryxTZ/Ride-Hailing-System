package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/compliance"
	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/messaging"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator/cets"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator/fake"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/sms"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Regulator adapter names (NEXA_REGULATOR_<NAME>_ADAPTER).
const (
	adapterPending = "pending" // queue only: nothing is sent until an adapter is configured
	adapterFake    = "fake"    // development and CI: scripted answers, nothing leaves the machine
	adapterCETS    = "cets"    // Tanzania's LATRA e-ticketing server
)

// complianceWorkers are registered before the River client exists.
type complianceWorkers struct {
	sweep     *compliance.SweepWorker
	daily     *compliance.DailyWorker
	documents *compliance.DocumentsWorker
}

// registerComplianceWorkers adds the regulator sweeper (every minute, the only
// retry clock) and the hourly daily-report check.
func registerComplianceWorkers(workers *river.Workers) (complianceWorkers, []*river.PeriodicJob) {
	cw := complianceWorkers{sweep: &compliance.SweepWorker{}, daily: &compliance.DailyWorker{}, documents: &compliance.DocumentsWorker{}}
	river.AddWorker(workers, cw.sweep)
	river.AddWorker(workers, cw.daily)
	river.AddWorker(workers, cw.documents)
	return cw, []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(compliance.SweepInterval), func() (river.JobArgs, *river.InsertOpts) {
			return compliance.SweepArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "compliance.regulator.sweep", RunOnStart: true}),
		river.NewPeriodicJob(river.PeriodicInterval(compliance.DailyInterval), func() (river.JobArgs, *river.InsertOpts) {
			return compliance.DailyArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "compliance.regulator.daily", RunOnStart: true}),
		river.NewPeriodicJob(river.PeriodicInterval(compliance.DocumentSweepInterval), func() (river.JobArgs, *river.InsertOpts) {
			return compliance.DocumentsArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "compliance.documents.sweep", RunOnStart: true}),
	}
}

// regulators builds one adapter per regulator the brand's countries name. Each
// country's regulator is independent; one with no adapter configured queues
// its submissions ("pending") until one is.
func regulators(bs *bootstrap) (map[string]regulator.Regulator, map[string]string, error) {
	regs, names := map[string]regulator.Regulator{}, map[string]string{}
	for _, c := range bs.brand.Countries {
		name := c.Regulator
		if name == "" || name == "none" || names[name] != "" {
			continue
		}
		adapter := strings.ToLower(os.Getenv("NEXA_REGULATOR_" + strings.ToUpper(name) + "_ADAPTER"))
		if adapter == "" {
			adapter = adapterPending
		}
		r, err := regulatorAdapter(bs, name, adapter)
		if err != nil {
			return nil, nil, fmt.Errorf("regulator %s: %w", name, err)
		}
		names[name] = adapter
		if r != nil {
			regs[name] = r
		}
		if adapter == adapterPending {
			slog.Warn("regulator submissions are queued but not sent", "regulator", name, "set", "NEXA_REGULATOR_"+strings.ToUpper(name)+"_ADAPTER")
		}
	}
	return regs, names, nil
}

// regulatorAdapter builds one adapter; real adapters exist per regulator (cets is LATRA's).
func regulatorAdapter(bs *bootstrap, name, adapter string) (regulator.Regulator, error) {
	switch adapter {
	case adapterPending:
		return nil, nil
	case adapterFake:
		f := fake.New(bs.cfg.FakeRegulatorMode)
		f.RegulatorName = name
		slog.Warn("regulator is the FAKE adapter: nothing reaches the regulator", "regulator", name)
		return f, nil
	case adapterCETS:
		if name != cets.Name {
			return nil, fmt.Errorf("the cets adapter is %s's, not %s's", cets.Name, name)
		}
		return cetsAdapter(bs)
	}
	return nil, fmt.Errorf("unknown adapter %q", adapter)
}

// cetsAdapter is LATRA's e-ticketing client; every setting is required.
func cetsAdapter(bs *bootstrap) (regulator.Regulator, error) {
	key, err := os.ReadFile(bs.cfg.LatraKeyFile)
	if err != nil {
		return nil, fmt.Errorf("NEXA_REGULATOR_LATRA_JWT_KEY_FILE: %w", err)
	}
	timeout, err := time.ParseDuration(bs.cfg.LatraTimeout)
	if err != nil {
		return nil, fmt.Errorf("NEXA_REGULATOR_LATRA_TIMEOUT: %w", err)
	}
	return cets.New(cets.Config{BaseURL: bs.cfg.LatraBaseURL, Issuer: bs.cfg.LatraIssuer, Audience: bs.cfg.LatraAudience,
		KID: bs.cfg.LatraKID, PrivateKeyPEM: key, JWKSURI: bs.cfg.LatraJWKSURI, Timeout: timeout})
}

// documentRules connects document expiry to holds (an expired blocking document
// holds the driver) and to reminders (push, plus SMS since the app may be closed).
func documentRules(svc *compliance.Service, f *fleet.Service, h *holds.Service, m *messaging.Service, text sms.Sender) {
	remind := func(ctx context.Context, driverID, phone, body string) error {
		if m != nil {
			m.NotifyDriver(ctx, driverID, "Document expiring", body, map[string]string{"type": "documents"})
		}
		return text.Send(ctx, phone, body)
	}
	svc.SetDocumentRules(h.SetDocumentHold, remind)
	f.OnDocumentsChanged(svc.EvaluateDriverDocuments)
}

// startCompliance wires regulator submissions: approval registers the driver and
// vehicle, completion submits the trip, fail-closed gates going online.
func startCompliance(bs *bootstrap, brandID string, cw complianceWorkers, cat *catalog.Catalog, t *trips.Service, f *fleet.Service) (*compliance.Service, error) {
	regs, names, err := regulators(bs)
	if err != nil {
		return nil, err
	}
	rules := func(ctx context.Context, serviceKey string) (json.RawMessage, error) {
		svc, err := cat.Find(ctx, serviceKey)
		if err != nil || svc == nil {
			return nil, err
		}
		return svc.Compliance, nil
	}
	svc := compliance.New(compliance.Deps{Pool: bs.pool, BrandID: brandID, Countries: bs.brand.Countries, Regulators: regs, Adapters: names, Rules: rules})
	cw.sweep.Svc, cw.daily.Svc, cw.documents.Svc = svc, svc, svc
	f.OnApprovedTx(svc.EnqueueApprovalTx)
	f.SetRegulatorGate(svc.Gate)
	t.OnCompletedTx(func(ctx context.Context, tx pgx.Tx, trip *trips.Trip) error {
		if trip.DriverID == nil {
			return nil
		}
		return svc.EnqueueTripTx(ctx, tx, trip.Country, trip.ServiceKey, trip.ID, *trip.DriverID)
	})
	return svc, nil
}
