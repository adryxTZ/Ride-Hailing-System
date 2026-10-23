package main

import (
	"context"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/safety"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// safetyTrips adapts trips (+ the driver's registered vehicle) to safety's Trips port.
type safetyTrips struct {
	t *trips.Service
	f *fleet.Service
}

func (a safetyTrips) RecordEvent(ctx context.Context, tripID, typ, actorKind string, actorID *string, payload any) error {
	return a.t.RecordEvent(ctx, tripID, typ, actorKind, actorID, payload)
}

func (a safetyTrips) Facts(ctx context.Context, id string) (*safety.TripFacts, error) {
	t, err := a.t.Load(ctx, id)
	if err != nil || t == nil {
		return nil, err
	}
	f := &safety.TripFacts{ID: t.ID, Code: t.Code, Status: string(t.Status), RiderID: t.RiderID, Terminal: trips.IsTerminal(t.Status)}
	if t.Rider != nil && t.Rider.Name != nil {
		f.RiderName = *t.Rider.Name
	}
	if t.DriverID != nil {
		f.DriverID = *t.DriverID
		if t.Driver != nil && t.Driver.Name != nil {
			f.DriverName = *t.Driver.Name
		}
		f.Vehicle = a.vehicleLabel(ctx, *t.DriverID)
	}
	for _, st := range t.Stops {
		p := &safety.Place{Location: st.Location}
		if st.Address != nil {
			p.Address = *st.Address
		}
		switch st.Kind {
		case "PICKUP":
			f.Pickup = p
		case "DROPOFF":
			f.Dropoff = p
		}
	}
	for _, k := range []string{"completed_at", "cancelled_at"} {
		if at := t.Timestamps[k]; at != nil {
			f.EndedAt = at
		}
	}
	return f, nil
}

// vehicleLabel is "Silver Toyota IST · T123ABC" from the registered vehicle.
func (a safetyTrips) vehicleLabel(ctx context.Context, driverID string) string {
	if a.f == nil {
		return ""
	}
	v, err := a.f.VehicleOf(ctx, driverID)
	if err != nil || v == nil {
		return ""
	}
	var parts []string
	for _, p := range []*string{v.Colour, v.Make, v.Model} {
		if p != nil && *p != "" {
			parts = append(parts, *p)
		}
	}
	if len(parts) == 0 {
		return v.Plate
	}
	return strings.Join(parts, " ") + " · " + v.Plate
}

// startSafety wires trusted contacts, share links and SOS.
func startSafety(bs *bootstrap, brandID string, t *trips.Service, f *fleet.Service, p *presence.Service) *safety.Service {
	var locate safety.DriverLocator
	if p != nil {
		locate = func(ctx context.Context, driverID string) *routing.Point {
			d, err := p.Get(ctx, driverID)
			if err != nil || d == nil {
				return nil
			}
			return &d.Location
		}
	}
	return safety.New(bs.pool, brandID, bs.brand.Country, bs.brand.DisplayName, safetyTrips{t: t, f: f}, locate, smsSender(bs))
}
