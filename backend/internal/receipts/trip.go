package receipts

import (
	"encoding/json"
	"time"

	ledger "github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// FactsFromTrip is a completed trip as a receipt needs it: the fare lines
// (quoted plus added during the ride), taxes from the split, how it was paid,
// and snapshots of the route and the driver's vehicle (plate, read by the caller).
func FactsFromTrip(t *trips.Trip, plate string) Facts {
	var b ledger.Breakdown
	_ = json.Unmarshal(t.Fees, &b)
	total := t.EstimatedFare
	if t.FinalFare != nil {
		total = *t.FinalFare
	}
	f := Facts{
		TripID: t.ID, RiderID: t.RiderID, Country: t.Country, Currency: t.Currency,
		Lines: fareLines(t), Total: total, VAT: b.VAT, Levy: b.Levy,
		Payment: Payment{Method: t.PaymentMethod, Status: deref(t.PaymentStatus), Reference: t.PaymentRef},
		Trip:    tripInfo(t),
	}
	if t.DriverID != nil {
		f.DriverID = *t.DriverID
		f.Driver = DriverInfo{Name: partyName(t.Driver), Plate: plate}
	}
	if f.Payment.Method == trips.MethodMobileMoney && f.Payment.Status == trips.PayCashConfirmed {
		f.Payment.Method = trips.MethodCash // the rider ended up paying cash
	}
	return f
}

// fareLines are the quoted lines followed by what was added during the ride.
func fareLines(t *trips.Trip) []FactLine {
	var quoted struct {
		Lines []FactLine `json:"lines"`
	}
	_ = json.Unmarshal(t.QuoteOption, &quoted)
	out := quoted.Lines
	for _, a := range t.Adjustments {
		out = append(out, FactLine{Code: a.Code, Amount: a.Amount, Meta: a.Meta})
	}
	return out
}

func tripInfo(t *trips.Trip) TripInfo {
	info := TripInfo{Code: t.Code, Service: t.ServiceKey, VehicleClass: t.VehicleClass,
		DistanceM: t.RouteDistanceM, DurationS: t.RouteDurationS, StartedAt: t.Timestamps["in_transit_at"], CompletedAt: time.Now().UTC()}
	if c := t.Timestamps["completed_at"]; c != nil {
		info.CompletedAt = *c
	}
	for _, s := range t.Stops {
		switch s.Kind {
		case "PICKUP":
			info.Pickup = deref(s.Address)
		case "DROPOFF":
			info.Dropoff = deref(s.Address)
		}
	}
	return info
}

func partyName(p *trips.Party) string {
	if p == nil || p.Name == nil {
		return ""
	}
	return *p.Name
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
