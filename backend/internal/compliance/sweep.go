package compliance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// dateLayout is the subject key of a daily report.
const dateLayout = "2006-01-02"

// Sweep sends every due submission of the countries that are sending (enabled,
// with a configured adapter). It is the only retry clock. Rows of other
// countries stay queued without counting attempts.
func (s *Service) Sweep(ctx context.Context) error {
	sending := s.sendingCountries(ctx)
	if len(sending) == 0 {
		return nil
	}
	due, err := s.store.claimDue(ctx, sending, SweepBatch)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, SendConcurrency)
	var wg sync.WaitGroup
	for _, c := range due {
		wg.Add(1)
		sem <- struct{}{}
		go func(c claimed) {
			defer func() { <-sem; wg.Done() }()
			if err := s.send(ctx, c); err != nil {
				slog.Warn("compliance: submission not recorded", "id", c.ID, "kind", c.Kind, "err", err)
			}
		}(c)
	}
	wg.Wait()
	return nil
}

// sendingCountries are the regulated countries whose switch is on and whose adapter is configured.
func (s *Service) sendingCountries(ctx context.Context) []string {
	var out []string
	for _, c := range s.countries {
		_, reg := s.regulated(c.Code)
		if reg == "" || s.regulators[reg] == nil {
			continue
		}
		if set, err := s.store.settings(ctx, c.Code); err == nil && set.Enabled {
			out = append(out, c.Code)
		}
	}
	return out
}

// send builds the record at send time, calls the regulator and records the answer.
func (s *Service) send(ctx context.Context, c claimed) error {
	country, reg := s.regulated(c.Country)
	adapter := s.regulators[reg]
	if country == nil || adapter == nil {
		return s.store.wait(ctx, c.ID, s.now().Add(SweepInterval))
	}
	if c.Kind == KindTrip {
		if ready, err := s.prerequisitesReady(ctx, reg, c); err != nil || !ready {
			return err
		}
	}
	payload, ack, err := s.deliver(ctx, adapter, country, c)
	if errors.Is(err, errLocalInvalid) {
		return s.store.fail(ctx, c.ID, codeLocalInvalid, err.Error(), NextAttempt(c.Attempts+1, s.now()), c.Attempts+1 >= StuckAfterAttempts)
	}
	if errors.Is(err, errMissing) {
		return s.store.skip(ctx, c.ID, "SUBJECT_MISSING", err.Error())
	}
	return s.record(ctx, c, reg, payload, ack, err)
}

// errLocalInvalid: our own data breaks the regulator's rules; not sent.
var errLocalInvalid = errors.New("compliance: record fails the country's rules")

// deliver builds the record for the submission's kind and sends it.
func (s *Service) deliver(ctx context.Context, r regulator.Regulator, c *brand.Country, sub claimed) (any, regulator.Ack, error) {
	switch sub.Kind {
	case KindDriver:
		d, err := s.store.driverRecord(ctx, sub.SubjectKey, c.Timezone)
		if err != nil {
			return nil, regulator.Ack{}, err
		}
		if !c.LicenceOK(d.LicenceNumber) {
			return nil, regulator.Ack{}, fmt.Errorf("%w: licence %q does not match %s", errLocalInvalid, d.LicenceNumber, c.LicencePattern)
		}
		ack, err := r.RegisterDriver(ctx, d)
		return d, ack, err
	case KindVehicle:
		v, err := s.store.vehicleRecord(ctx, sub.SubjectKey, c.Timezone)
		if err != nil {
			return nil, regulator.Ack{}, err
		}
		ack, err := r.RegisterVehicle(ctx, v)
		return v, ack, err
	case KindTrip:
		t, err := s.store.tripRecord(ctx, sub.SubjectKey, c.Timezone)
		if err != nil {
			return nil, regulator.Ack{}, err
		}
		ack, err := r.SubmitTrip(ctx, t)
		return t, ack, err
	}
	return s.deliverReport(ctx, r, c, sub)
}

// deliverReport sends a country day's totals; a day with no trips closes locally.
func (s *Service) deliverReport(ctx context.Context, r regulator.Regulator, c *brand.Country, sub claimed) (any, regulator.Ack, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, regulator.Ack{}, err
	}
	day, err := time.ParseInLocation(dateLayout, sub.SubjectKey, loc)
	if err != nil {
		return nil, regulator.Ack{}, err
	}
	n, paid, err := s.store.dayTotals(ctx, c.Code, sub.Regulator, day, day.AddDate(0, 0, 1))
	if err != nil {
		return nil, regulator.Ack{}, err
	}
	rep := regulator.DailyReport{Country: c.Code, Date: day, Timezone: c.Timezone, Trips: n, Paid: paid, Currency: c.Currency}
	if n == 0 {
		return rep, regulator.Ack{}, &regulator.Rejection{Code: "NO_TRIPS", Message: "no trips that day", Class: regulator.ClassNotNeeded}
	}
	ack, err := r.SubmitDailyReport(ctx, rep)
	return rep, ack, err
}

// record applies the regulator's answer to the submission.
func (s *Service) record(ctx context.Context, c claimed, reg string, payload any, ack regulator.Ack, err error) error {
	o := outcomeOf(ack, err)
	switch o.status {
	case StatusAccepted:
		if c.DriverID != nil && (c.Kind == KindDriver || c.Kind == KindVehicle) {
			s.Forget(*c.DriverID)
		}
		return s.store.accept(ctx, c.ID, ack.Code, ack.Reference, payload)
	case StatusSkipped:
		return s.store.skip(ctx, c.ID, o.code, o.msg)
	}
	attempts := c.Attempts + 1
	if attempts == StuckAfterAttempts {
		slog.Error("compliance: submission stuck at the regulator", "id", c.ID, "kind", c.Kind, "subject", c.SubjectKey, "code", o.code, "error", o.msg)
	}
	if o.prerequisite {
		if perr := s.reregister(ctx, c, reg); perr != nil {
			slog.Warn("compliance: re-registration not queued", "id", c.ID, "err", perr)
		}
	}
	return s.store.fail(ctx, c.ID, o.code, o.msg, NextAttempt(attempts, s.now()), attempts >= StuckAfterAttempts)
}

// prerequisitesReady holds a trip until its driver and vehicle are registered,
// bringing those forward so the trip is not left waiting a ladder rung.
func (s *Service) prerequisitesReady(ctx context.Context, reg string, c claimed) (bool, error) {
	if c.DriverID == nil {
		return true, nil
	}
	vehicle, err := s.tripVehicle(ctx, c)
	if err != nil {
		return false, err
	}
	ready := true
	for kind, key := range map[string]string{KindDriver: *c.DriverID, KindVehicle: vehicle} {
		st, err := s.store.statusOf(ctx, reg, kind, key)
		if err != nil {
			return false, err
		}
		if st != StatusAccepted {
			ready = false
			if st == "" {
				return false, s.reregister(ctx, c, reg)
			}
			if err := s.store.makeDue(ctx, reg, kind, key); err != nil {
				return false, err
			}
		}
	}
	if !ready {
		return false, s.store.wait(ctx, c.ID, s.now().Add(SweepInterval))
	}
	return true, nil
}

// tripVehicle is the vehicle the driver used (their current one).
func (s *Service) tripVehicle(ctx context.Context, c claimed) (string, error) {
	if c.VehicleID != nil {
		return *c.VehicleID, nil
	}
	var id string
	err := s.store.pool.QueryRow(ctx, `SELECT id FROM vehicles WHERE driver_id=$1 ORDER BY created_at DESC LIMIT 1`, *c.DriverID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// reregister queues the driver and vehicle registrations again (the regulator does not know them).
func (s *Service) reregister(ctx context.Context, c claimed, reg string) error {
	if c.DriverID == nil {
		return nil
	}
	vehicle, err := s.tripVehicle(ctx, c)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		return s.EnqueueApprovalTx(ctx, tx, c.Country, *c.DriverID, vehicle)
	})
}

// DailyReports queues yesterday's report for each regulated country once its
// local clock has passed DailyReportAt. A day is queued once and never recomputed after it is sent.
func (s *Service) DailyReports(ctx context.Context) error {
	for _, c := range s.countries {
		_, reg := s.regulated(c.Code)
		if reg == "" || c.Status != brand.CountryActive {
			continue
		}
		start := midnight(s.now(), c.Timezone)
		if s.now().Before(start.Add(DailyReportAt)) {
			continue
		}
		day := start.AddDate(0, 0, -1).Format(dateLayout)
		if err := s.store.enqueueOnceTx(ctx, s.store.pool, subject{country: c.Code, regulator: reg, kind: KindDailyReport, key: day}); err != nil {
			return err
		}
	}
	return nil
}
