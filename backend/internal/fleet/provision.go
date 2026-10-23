package fleet

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Provision registers a driver and vehicle as already APPROVED, without
// documents: for operator tooling and test environments (e.g. a fleet's
// paper-checked drivers). Audited as driver.provision.
func (s *Service) Provision(ctx context.Context, actorID *string, userID string, p ProfileInput, v VehicleInput) (*Onboarding, error) {
	v.Plate = normalisePlate(v.Plate)
	ok, err := s.classes.ClassExists(ctx, v.ServiceKey, v.ClassKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrUnknownClass
	}
	if err := s.store.ensureProfile(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.store.saveProfile(ctx, userID, p); err != nil {
		return nil, err
	}
	veh, err := s.store.saveVehicle(ctx, userID, v)
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.setVehicleStatus(ctx, tx, veh.ID, ReviewApproved, nil); err != nil {
			return err
		}
		if err := s.store.setStatus(ctx, tx, userID, StatusApproved, nil, actorID); err != nil {
			return err
		}
		if err := s.approvedTx(ctx, tx, userID, veh.ID); err != nil {
			return err
		}
		actor := "" // operator command: no staff account behind it
		if actorID != nil {
			actor = *actorID
		}
		return s.store.audit(ctx, tx, actor, "driver.provision", "driver:"+userID, nil, map[string]string{"plate": veh.Plate, "class": v.ClassKey})
	})
	if err != nil {
		return nil, err
	}
	return s.assemble(ctx, userID)
}
