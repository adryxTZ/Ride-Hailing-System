package main

import (
	"context"
	"os"

	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/messaging"
	"github.com/kalen-limited/hauler-rider/backend/internal/realtime"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// tripViews adapts the trips service to messaging's narrow Trips port.
type tripViews struct{ t *trips.Service }

func (a tripViews) View(ctx context.Context, id string) (*messaging.TripView, error) {
	t, err := a.t.Load(ctx, id)
	if err != nil || t == nil {
		return nil, err
	}
	v := &messaging.TripView{ID: t.ID, Code: t.Code, Status: string(t.Status), RiderID: t.RiderID, Terminal: trips.IsTerminal(t.Status)}
	if t.DriverID != nil {
		v.DriverID = *t.DriverID
	}
	return v, nil
}

// hubSender adapts the realtime hub to messaging's Realtime port.
type hubSender struct{ h *realtime.Hub }

func (a hubSender) Send(ctx context.Context, userID, typ string, data any) {
	a.h.Publish(ctx, userID, realtime.NewFrame(typ, data))
}

// pusher picks push delivery: Expo's push service unless NEXA_PUSH=off.
func pusher() messaging.Pusher {
	if os.Getenv("NEXA_PUSH") == "off" {
		return messaging.LogPusher{}
	}
	return messaging.NewExpoPusher(os.Getenv("NEXA_EXPO_ACCESS_TOKEN"))
}

// startMessaging wires chat/ratings/push and subscribes it to trip changes and offers.
func startMessaging(bs *bootstrap, brandID string, t *trips.Service, hub *realtime.Hub, d *dispatch.Service) *messaging.Service {
	var rt messaging.Realtime
	if hub != nil {
		rt = hubSender{h: hub}
	}
	m := messaging.New(bs.pool, brandID, tripViews{t: t}, rt, pusher())
	t.OnChanged(func(ctx context.Context, trip *trips.Trip) {
		c := messaging.TripChange{ID: trip.ID, Code: trip.Code, Status: string(trip.Status), RiderID: trip.RiderID, Scheduled: trip.ScheduledFor != nil}
		if trip.DriverID != nil {
			c.DriverID = *trip.DriverID
		}
		if trip.Driver != nil && trip.Driver.Name != nil {
			c.DriverName = *trip.Driver.Name
		}
		go m.TripChanged(context.WithoutCancel(ctx), c) // push never slows the request
	})
	if d != nil {
		d.OnOffer(func(ctx context.Context, driverID string, o *dispatch.Offer) {
			p := messaging.OfferPush{TripID: o.TripID, Fare: o.Fare.StringFixed(0), Currency: o.Currency}
			if o.Pickup.Address != nil {
				p.Pickup = *o.Pickup.Address
			}
			go m.OfferMade(context.WithoutCancel(ctx), driverID, p)
		})
	}
	return m
}
