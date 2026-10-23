package main

import (
	"context"

	"github.com/kalen-limited/hauler-rider/backend/internal/areas"
	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// startAreas builds the operating-areas service and plugs it into pricing (a
// quote is priced for the area its pickup falls in) and fleet (a driver's areas
// travel with their eligibility; sign-up records the first one).
func startAreas(bs *bootstrap, brandID string, prices *pricing.Service, fleetSvc *fleet.Service) *areas.Service {
	svc := areas.New(bs.pool, brandID, bs.brand, audit.Record)
	prices.SetAreas(areaResolver{svc})
	if fleetSvc != nil {
		fleetSvc.SetAreas(driverAreas{svc})
	}
	return svc
}

// driverAreas adapts the areas service to fleet's port.
type driverAreas struct{ svc *areas.Service }

func (d driverAreas) AreaIDs(ctx context.Context, driverID string) ([]string, error) {
	refs, _, err := d.svc.DriverAreas(ctx, driverID)
	ids := make([]string, len(refs))
	for i, r := range refs {
		ids[i] = r.ID
	}
	return ids, err
}

func (d driverAreas) SetSignupArea(ctx context.Context, driverID, areaID string) error {
	_, err := d.svc.SetDriverAreas(ctx, "", driverID, []string{areaID}, areaID, "")
	return err
}

// areaResolver adapts the areas service to pricing's port.
type areaResolver struct{ svc *areas.Service }

func (r areaResolver) Serviceable(ctx context.Context, p routing.Point) (pricing.AreaRule, error) {
	ref, err := r.svc.Resolve(ctx, p)
	if err != nil {
		return pricing.AreaRule{}, err
	}
	ix, err := r.svc.Index(ctx)
	if err != nil {
		return pricing.AreaRule{}, err
	}
	rule := pricing.AreaRule{Ref: pricing.AreaRef(ref), SurgeEnabled: true}
	if a, ok := ix.Area(ref.ID); ok {
		rule.SurgeEnabled, rule.SurgeMax = a.SurgeEnabled, a.SurgeMax
	}
	return rule, nil
}

func (r areaResolver) AreaOf(ctx context.Context, p routing.Point) (pricing.AreaRef, bool, error) {
	ix, err := r.svc.Index(ctx)
	if err != nil {
		return pricing.AreaRef{}, false, err
	}
	ref, ok := ix.AreaOf(p)
	return pricing.AreaRef(ref), ok, nil
}
