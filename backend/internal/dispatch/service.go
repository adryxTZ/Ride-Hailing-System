package dispatch

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/realtime"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Service runs matching and answers drivers' accept / decline.
type Service struct {
	store    store
	trips    *trips.Service
	presence *presence.Service
	hub      *realtime.Hub
	routing  routing.Provider
	redis    *redis.Client
	brand    string
	instance string
	now      func() time.Time
	lastGC   time.Time
	onOffer  []func(ctx context.Context, driverID string, o *Offer)
	// periodic runs on the leader every periodicInterval (e.g. surge).
	periodic []func(ctx context.Context)
	lastRun  time.Time
}

// periodicInterval is how often leader-only background work runs.
const periodicInterval = 30 * time.Second

// Every30s registers work only the dispatching leader does, twice a minute.
func (s *Service) Every30s(fn func(ctx context.Context)) { s.periodic = append(s.periodic, fn) }

// OnOffer registers a listener told about every new offer (e.g. a push that wakes
// the driver's phone when the socket is asleep).
func (s *Service) OnOffer(fn func(ctx context.Context, driverID string, o *Offer)) {
	s.onOffer = append(s.onOffer, fn)
}

func New(pool *pgxpool.Pool, r *redis.Client, brand string, t *trips.Service, p *presence.Service, hub *realtime.Hub, rt routing.Provider) *Service {
	return &Service{store: store{pool: pool}, trips: t, presence: p, hub: hub, routing: rt, redis: r, brand: brand, instance: uuid.NewString(), now: time.Now}
}

// Run ticks until ctx ends. Only the process holding the brand's lock dispatches.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(TickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.lead(ctx) {
				continue
			}
			if err := s.tick(ctx); err != nil {
				slog.Warn("dispatch tick failed", "err", err)
			}
		}
	}
}

// lead takes or renews the per-brand dispatcher lock.
func (s *Service) lead(ctx context.Context) bool {
	key := "dispatch:leader:" + s.brand
	ok, err := s.redis.SetNX(ctx, key, s.instance, leaderTTL).Result()
	if err == nil && ok {
		return true
	}
	if holder, _ := s.redis.Get(ctx, key).Result(); holder == s.instance {
		s.redis.PExpire(ctx, key, leaderTTL)
		return true
	}
	return false
}

func (s *Service) tick(ctx context.Context) error {
	if s.now().Sub(s.lastGC) >= presence.GCInterval {
		s.lastGC = s.now()
		if _, err := s.presence.GC(ctx); err != nil {
			slog.Warn("presence gc failed", "err", err)
		}
	}
	if s.now().Sub(s.lastRun) >= periodicInterval {
		s.lastRun = s.now()
		for _, fn := range s.periodic {
			fn(ctx)
		}
	}
	if err := s.expireOffers(ctx); err != nil {
		return err
	}
	started, err := s.trips.ActivateScheduled(ctx)
	if err != nil {
		return err
	}
	for _, id := range started {
		s.trips.Notify(ctx, id) // rider hears "finding your driver"; matching picks it up below
	}
	ended, err := s.trips.ExpireUnmatched(ctx)
	if err != nil {
		return err
	}
	for _, id := range ended {
		s.closeOffersFor(ctx, id)
		s.trips.Notify(ctx, id)
	}
	return s.match(ctx)
}

// match finds candidates for every waiting trip and offers the batch assignment.
func (s *Service) match(ctx context.Context) error {
	open, err := s.trips.OpenRequests(ctx, BatchLimit)
	if err != nil || len(open) == 0 {
		return err
	}
	ids := make([]string, len(open))
	for i, r := range open {
		ids[i] = r.TripID
	}
	asked, err := s.store.excluded(ctx, ids)
	if err != nil {
		return err
	}
	var cands []candidate
	for _, r := range open {
		cands = append(cands, s.candidates(ctx, r, asked[r.TripID])...)
	}
	for _, c := range assign(ids, cands) {
		s.offer(ctx, c)
	}
	return nil
}

// candidates widens the search ring until idle, eligible drivers appear.
func (s *Service) candidates(ctx context.Context, r trips.OpenRequest, asked map[string]bool) []candidate {
	for _, k := range ringSteps {
		drivers, err := s.presence.Nearby(ctx, r.Pickup, k, r.ServiceKey, r.VehicleClass, r.AreaID, CandidateCap*2)
		if err != nil {
			slog.Warn("dispatch nearby failed", "err", err)
			return nil
		}
		var out []candidate
		for _, d := range drivers {
			dist := routing.HaversineM(r.Pickup, d.Location)
			if !offerable(d, r, asked, dist) {
				continue
			}
			out = append(out, candidate{TripID: r.TripID, DriverID: d.ID, DistanceM: dist, EtaS: etaSeconds(dist)})
			if len(out) == CandidateCap {
				break
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// offerable reports whether a nearby driver may be offered a request: idle,
// not already asked, close enough, and never the rider's own account (a person
// signed in as both rider and driver would otherwise be offered their own ride).
func offerable(d presence.Driver, r trips.OpenRequest, asked map[string]bool, distM float64) bool {
	return d.State == presence.Idle && !asked[d.ID] && distM <= maxPickupKm*1000 && d.ID != r.RiderID
}

// offer records the offer and flips the trip to OFFERED atomically, then tells the driver.
func (s *Service) offer(ctx context.Context, c candidate) {
	c = s.withRoadETA(ctx, c)
	expires := s.now().Add(OfferTTL)
	committed := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.insertOffer(ctx, tx, c, expires)
		if err != nil || !ok {
			return err
		}
		committed, err = s.trips.MarkOffered(ctx, tx, c.TripID, c.DriverID)
		if err == nil && !committed {
			return pgx.ErrTxClosed // trip moved on; roll the offer back
		}
		return err
	})
	if err != nil || !committed {
		return
	}
	t, err := s.trips.Load(ctx, c.TripID)
	if err != nil || t == nil {
		return
	}
	_ = s.presence.SetState(ctx, c.DriverID, presence.Offered, c.TripID, t.RiderID)
	o := buildOffer(t, c.DistanceM, c.EtaS, expires)
	s.hub.Publish(ctx, c.DriverID, realtime.NewFrame(realtime.TypeOfferNew, o))
	for _, fn := range s.onOffer {
		fn(ctx, c.DriverID, o)
	}
	s.trips.Notify(ctx, c.TripID)
}

// expireOffers turns timed-out offers into TIMED_OUT and frees trip and driver.
func (s *Service) expireOffers(ctx context.Context) error {
	var gone []openOffer
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		var err error
		if gone, err = s.store.expired(ctx, tx); err != nil {
			return err
		}
		for _, o := range gone {
			if err := s.trips.Release(ctx, tx, o.TripID, "timed_out"); err != nil {
				return err
			}
		}
		return nil
	})
	for _, o := range gone {
		s.freeDriver(ctx, o.DriverID, o.TripID, "timed_out")
		s.trips.Notify(ctx, o.TripID)
	}
	return err
}

// Accept assigns the trip to the driver if their offer is still open.
func (s *Service) Accept(ctx context.Context, tripID, driverID string) (*trips.Trip, error) {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		o, err := s.store.lockOpen(ctx, tx, tripID, driverID)
		if err != nil {
			return err
		}
		if o == nil {
			return ErrOfferNotFound
		}
		if !s.now().Before(o.ExpiresAt) {
			return ErrOfferExpired
		}
		if err := s.store.close(ctx, tx, tripID, driverID, "ACCEPTED"); err != nil {
			return err
		}
		return s.trips.Assign(ctx, tx, tripID, driverID)
	})
	if err != nil {
		return nil, err
	}
	t, err := s.trips.Load(ctx, tripID)
	if err != nil || t == nil {
		return t, err
	}
	_ = s.presence.SetState(ctx, driverID, presence.OnTrip, tripID, t.RiderID)
	s.trips.Notify(ctx, tripID)
	return t, nil
}

// Decline frees the trip for the next driver; this driver is not asked again for it.
func (s *Service) Decline(ctx context.Context, tripID, driverID string) error {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		o, err := s.store.lockOpen(ctx, tx, tripID, driverID)
		if err != nil {
			return err
		}
		if o == nil {
			return ErrOfferNotFound
		}
		if err := s.store.close(ctx, tx, tripID, driverID, "DECLINED"); err != nil {
			return err
		}
		return s.trips.Release(ctx, tx, tripID, "declined")
	})
	if err != nil {
		return err
	}
	s.freeDriver(ctx, driverID, tripID, "declined")
	s.trips.Notify(ctx, tripID)
	return nil
}

// Current returns the driver's open offer (for the app after a restart), or nil.
func (s *Service) Current(ctx context.Context, driverID string) (*Offer, error) {
	o, err := s.store.openForDriver(ctx, driverID)
	if err != nil || o == nil {
		return nil, err
	}
	t, err := s.trips.Load(ctx, o.TripID)
	if err != nil || t == nil {
		return nil, err
	}
	return buildOffer(t, float64(o.DistanceM), o.EtaS, o.ExpiresAt), nil
}

// TripChanged keeps drivers in step with trip changes made elsewhere: a cancelled
// or finished trip closes its open offer and returns the driver to IDLE.
func (s *Service) TripChanged(ctx context.Context, t *trips.Trip) {
	if !trips.IsTerminal(t.Status) && t.Status != trips.Requested {
		return
	}
	s.closeOffersFor(ctx, t.ID)
	if t.DriverID != nil {
		_ = s.presence.SetState(ctx, *t.DriverID, presence.Idle, "", "")
	}
}

func (s *Service) closeOffersFor(ctx context.Context, tripID string) {
	driver, err := s.store.cancelForTrip(ctx, tripID)
	if err == nil && driver != "" {
		s.freeDriver(ctx, driver, tripID, "cancelled")
	}
}

func (s *Service) freeDriver(ctx context.Context, driverID, tripID, reason string) {
	_ = s.presence.SetState(ctx, driverID, presence.Idle, "", "")
	s.hub.Publish(ctx, driverID, realtime.NewFrame(realtime.TypeOfferClosed, map[string]string{"trip_id": tripID, "reason": reason}))
}

func buildOffer(t *trips.Trip, distM float64, etaS int, expires time.Time) *Offer {
	o := &Offer{
		TripID: t.ID, ExpiresAt: expires, TTLSeconds: int(time.Until(expires).Seconds()), Service: t.ServiceKey,
		VehicleClass: t.VehicleClass, Fare: t.EstimatedFare, Currency: t.Currency, PaymentMethod: t.PaymentMethod,
		PickupDistance: int(distM), PickupEtaS: etaS, TripDistanceM: t.RouteDistanceM,
	}
	if t.Rider != nil {
		o.RiderName = t.Rider.Name
	}
	for _, st := range t.Stops {
		stop := OfferStop{Location: st.Location, Address: st.Address}
		switch st.Kind {
		case "PICKUP":
			o.Pickup = stop
		case "DROPOFF":
			o.Dropoff = &stop
		case "WAYPOINT":
			o.StopsCount++
		}
	}
	return o
}

// withRoadETA replaces the straight-line estimate with the road route for the
// chosen driver only: one (cached) routing call per offer, never per candidate.
func (s *Service) withRoadETA(ctx context.Context, c candidate) candidate {
	if s.routing == nil {
		return c
	}
	d, err := s.presence.Get(ctx, c.DriverID)
	if err != nil || d == nil {
		return c
	}
	open, err := s.trips.Load(ctx, c.TripID)
	if err != nil || open == nil || len(open.Stops) == 0 {
		return c
	}
	if r, err := s.routing.Route(ctx, d.Location, open.Stops[0].Location); err == nil && r.DurationS > 0 {
		c.DistanceM, c.EtaS = float64(r.DistanceM), r.DurationS
	}
	return c
}
