package dispatch

import (
	"context"
	"log/slog"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Trip queue (forward dispatch): a driver whose current trip is finishing and
// who has no next trip yet is a candidate for requests near their drop-off.
// Everything matching needs is in their presence record (finishing flag,
// drop-off point, queued trip), so matching stays in Redis.
const (
	// finishingAllowanceS is the time a finishing driver still needs before
	// leaving the drop-off (the rider getting out, the fare being paid). It is
	// added to their pickup ETA, so riders and the batch see the real wait.
	finishingAllowanceS = 120
	// queuedRankPenaltyS ranks a finishing driver behind an idle one unless
	// they are clearly closer: an idle driver whose ETA is within this of the
	// finishing driver's (allowance included) gets the trip.
	queuedRankPenaltyS = 120
)

// queueRule tells matching whether a request's country lets finishing drivers
// take the next trip.
func (s *Service) queueRule(ctx context.Context, country string) bool {
	return s.Settings().For(ctx, country).Enabled
}

// queueable reports whether a finishing driver may be offered a request as
// their next trip: finishing, nothing queued yet, never the rider's own
// account, not asked before, and the pickup close enough to their drop-off.
func queueable(d presence.Driver, r trips.OpenRequest, asked map[string]bool, distM float64) bool {
	return d.State == presence.OnTrip && d.Finishing && d.QueuedTripID == "" &&
		!asked[d.ID] && distM <= maxPickupKm*1000 && d.ID != r.RiderID
}

// queuedOrigin is where a finishing driver's next pickup is measured from:
// the current trip's drop-off, else where they are now.
func queuedOrigin(d presence.Driver) routing.Point {
	if d.Dropoff != nil {
		return *d.Dropoff
	}
	return d.Location
}

// candidateOf turns a nearby driver into a candidate for r, idle or finishing.
func candidateOf(d presence.Driver, r trips.OpenRequest, asked map[string]bool, queueOn bool) (candidate, bool) {
	if d.State == presence.Idle {
		dist := routing.HaversineM(r.Pickup, d.Location)
		return candidate{TripID: r.TripID, DriverID: d.ID, DistanceM: dist, EtaS: etaSeconds(dist)}, offerable(d, r, asked, dist)
	}
	if !queueOn {
		return candidate{}, false
	}
	dist := routing.HaversineM(r.Pickup, queuedOrigin(d))
	c := candidate{TripID: r.TripID, DriverID: d.ID, DistanceM: dist, EtaS: etaSeconds(dist) + finishingAllowanceS, Queued: true}
	return c, queueable(d, r, asked, dist)
}

// markFinishing flags the driver of a trip that reached a finishing status of
// its country, with the drop-off point; other statuses leave presence alone.
func (s *Service) markFinishing(ctx context.Context, t *trips.Trip) {
	if t.DriverID == nil || t.Queued || !s.Settings().For(ctx, t.Country).IsFinishing(t.Status) {
		return
	}
	for _, st := range t.Stops {
		if st.Kind == "DROPOFF" {
			if err := s.presence.SetFinishing(ctx, *t.DriverID, t.ID, st.Location); err != nil {
				slog.Warn("presence finishing failed", "driver", *t.DriverID, "err", err)
			}
			return
		}
	}
}

// DriverLeftTrip updates the driver's presence after tripID ended for them
// (completed, cancelled, or sent back to matching): a queued trip leaves the
// queue; the current trip hands over to the next one, or the driver is IDLE.
// Safe to call more than once for the same trip.
func (s *Service) DriverLeftTrip(ctx context.Context, driverID, tripID string) {
	d, err := s.presence.Get(ctx, driverID)
	if err != nil || d == nil {
		return
	}
	if d.QueuedTripID == tripID {
		_ = s.presence.ReleaseOffer(ctx, driverID, tripID)
		return
	}
	if d.TripID == tripID {
		s.moveToNext(ctx, d)
	}
}

// moveToNext hands a driver whose current trip ended to their next one. The
// database promoted it in the ending transaction; presence follows here.
func (s *Service) moveToNext(ctx context.Context, d *presence.Driver) {
	next, err := s.trips.ActiveForDriver(ctx, d.ID)
	if err == nil && next != nil && next.ID != d.TripID {
		_ = s.presence.Promote(ctx, d.ID, d.TripID, next.ID, next.RiderID, presence.OnTrip)
		s.trips.Notify(ctx, next.ID) // both apps hear the next trip is current
		return
	}
	if d.QueuedTripID != "" { // an offer still open: it becomes a plain offer
		if o, err := s.trips.Load(ctx, d.QueuedTripID); err == nil && o != nil && o.Status == trips.Offered {
			_ = s.presence.Promote(ctx, d.ID, d.TripID, o.ID, o.RiderID, presence.Offered)
			return
		}
	}
	_ = s.presence.SetState(ctx, d.ID, presence.Idle, "", "")
}

// DriverOnline puts a driver who comes back online onto the trip they are
// still on (presence starts IDLE), finishing flag included, so they are not
// offered a second current trip.
func (s *Service) DriverOnline(ctx context.Context, driverID string) {
	t, err := s.trips.ActiveForDriver(ctx, driverID)
	if err != nil || t == nil {
		return
	}
	_ = s.presence.SetState(ctx, driverID, presence.OnTrip, t.ID, t.RiderID)
	s.markFinishing(ctx, t)
}

// DriverOffline releases the queued trip of a driver who went offline (or was
// swept as stale) back to matching; an open queued offer just expires.
func (s *Service) DriverOffline(ctx context.Context, d presence.Driver) {
	if d.QueuedTripID == "" {
		return
	}
	released, err := s.trips.ReleaseQueued(ctx, d.QueuedTripID, d.ID)
	if err != nil {
		slog.Warn("release queued trip failed", "driver", d.ID, "trip", d.QueuedTripID, "err", err)
		return
	}
	if released {
		s.trips.Notify(ctx, d.QueuedTripID)
	}
}

// NextTrip is the driver's queued trip as the "Next trip" banner shows it.
type NextTrip struct {
	TripID          string    `json:"trip_id"`
	Code            string    `json:"code"`
	Pickup          OfferStop `json:"pickup"`
	RiderFirstName  *string   `json:"rider_first_name,omitempty"`
	PickupDistanceM int       `json:"pickup_distance_m"`
	PickupEtaS      *int      `json:"pickup_eta_s,omitempty"`
}

// NextTrip returns the driver's queued trip, or nil. The distance is measured
// from the current drop-off (presence), else from where the driver is.
func (s *Service) NextTrip(ctx context.Context, driverID string) (*NextTrip, error) {
	t, err := s.trips.QueuedForDriver(ctx, driverID)
	if err != nil || t == nil {
		return nil, err
	}
	n := &NextTrip{TripID: t.ID, Code: t.Code, PickupEtaS: t.QueuedPickupEtaS}
	for _, st := range t.Stops {
		if st.Kind == "PICKUP" {
			n.Pickup = OfferStop{Location: st.Location, Address: st.Address}
		}
	}
	if t.Rider != nil {
		n.RiderFirstName = firstName(t.Rider.Name)
	}
	if d, err := s.presence.Get(ctx, driverID); err == nil && d != nil {
		n.PickupDistanceM = int(routing.HaversineM(queuedOrigin(*d), n.Pickup.Location))
	}
	return n, nil
}

// firstName is the first word of a full name (the banner is compact and the
// driver needs no more of a rider they have not met).
func firstName(full *string) *string {
	if full == nil {
		return nil
	}
	parts := strings.Fields(*full)
	if len(parts) == 0 {
		return nil
	}
	return &parts[0]
}
