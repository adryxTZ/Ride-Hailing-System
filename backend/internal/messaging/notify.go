package messaging

import (
	"context"
	"fmt"
)

// TripChange is a committed trip change as notifications need it.
type TripChange struct {
	ID         string
	Code       string
	Status     string
	RiderID    string
	DriverID   string
	DriverName string
	// Scheduled is true for rides booked ahead (their REQUESTED is worth a push).
	Scheduled bool
}

// OfferPush is a new offer as the push needs it.
type OfferPush struct {
	TripID   string
	Pickup   string
	Fare     string
	Currency string
}

// tripPush is the rider-facing and driver-facing text for a status, when there is one.
type tripPush struct{ rider, driver string }

var tripPushes = map[string]tripPush{
	"ACCEPTED":            {rider: "%s is on the way to pick you up."},
	"ARRIVED_AT_PICKUP":   {rider: "Your driver has arrived."},
	"AWAITING_PAYMENT":    {rider: "You have arrived. Please pay your driver."},
	"COMPLETED":           {rider: "Thanks for travelling with us. Rate your trip."},
	"NO_DRIVER_FOUND":     {rider: "No driver was available. Please try again."},
	"EXPIRED":             {rider: "Your scheduled ride could not start. Please book again."},
	"CANCELLED_BY_DRIVER": {rider: "Your driver cancelled. We are finding you another one."},
	"CANCELLED_BY_RIDER":  {driver: "The rider cancelled trip %s."},
}

// TripChanged turns a status change into push notifications for whoever needs one.
func (s *Service) TripChanged(ctx context.Context, c TripChange) {
	p, ok := tripPushes[c.Status]
	if c.Status == "REQUESTED" && c.Scheduled {
		p, ok = tripPush{rider: "It's almost time for your scheduled ride. We're finding your driver."}, true
	}
	if !ok {
		return
	}
	data := map[string]string{"type": "trip", "trip_id": c.ID, "status": c.Status}
	if p.rider != "" {
		body := p.rider
		if c.Status == "ACCEPTED" {
			body = fmt.Sprintf(p.rider, nameOr(c.DriverName, "Your driver"))
		}
		s.push(ctx, c.RiderID, RoleRider, Notification{Title: "Trip " + c.Code, Body: body, Data: data, Channel: "trips"})
	}
	if p.driver != "" && c.DriverID != "" {
		s.push(ctx, c.DriverID, RoleDriver, Notification{Title: "Trip update", Body: fmt.Sprintf(p.driver, c.Code), Data: data, Channel: "trips"})
	}
}

// OfferMade wakes the driver's phone with a high-priority push for a new offer.
func (s *Service) OfferMade(ctx context.Context, driverID string, o OfferPush) {
	s.push(ctx, driverID, RoleDriver, Notification{
		Title:    "New trip request",
		Body:     fmt.Sprintf("Pickup: %s · %s %s", nameOr(o.Pickup, "nearby"), o.Currency, o.Fare),
		Data:     map[string]string{"type": "offer", "trip_id": o.TripID},
		Priority: "high",
		Channel:  "offers",
	})
}

func nameOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// NotifyDriver sends one push to a driver's app (account notices such as document expiry).
func (s *Service) NotifyDriver(ctx context.Context, driverID, title, body string, data map[string]string) {
	s.push(ctx, driverID, RoleDriver, Notification{Title: title, Body: body, Data: data, Channel: "account"})
}
