package compliance

import (
	"context"
	"time"
)

// gateEntry is a cached fail-closed answer.
type gateEntry struct {
	err error
	at  time.Time
}

// Gate refuses to let a driver go online while the regulator has not accepted
// their registration and their vehicle's, when the country runs fail-closed.
// Answers are cached per driver for gateTTL; a ride is never blocked by trip submission.
func (s *Service) Gate(ctx context.Context, country, driverID, vehicleID string) error {
	if v, ok := s.gate.Load(driverID); ok {
		if e := v.(gateEntry); s.now().Sub(e.at) < gateTTL {
			return e.err
		}
	}
	err := s.gateFor(ctx, country, driverID, vehicleID)
	if err == nil || err == ErrRegistrationOpen {
		s.gate.Store(driverID, gateEntry{err: err, at: s.now()})
	}
	return err
}

func (s *Service) gateFor(ctx context.Context, country, driverID, vehicleID string) error {
	_, reg := s.regulated(country)
	if reg == "" {
		return nil
	}
	set, err := s.store.settings(ctx, country)
	if err != nil || !set.FailClosed {
		return err
	}
	for kind, key := range map[string]string{KindDriver: driverID, KindVehicle: vehicleID} {
		st, err := s.store.statusOf(ctx, reg, kind, key)
		if err != nil {
			return err
		}
		if st != StatusAccepted {
			return ErrRegistrationOpen
		}
	}
	return nil
}

// Forget drops a driver's cached gate answer (after their registration is accepted).
func (s *Service) Forget(driverID string) { s.gate.Delete(driverID) }
