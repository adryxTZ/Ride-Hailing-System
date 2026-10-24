package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing/engine"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Service is the quote use case for one brand.
type Service struct {
	areas   AreaResolver // nil until SetAreas
	store   store
	catalog *catalog.Catalog
	routing routing.Provider
	cache   *cache.Versioned[tariffs]
	schemas *schemaCache
	now     func() time.Time
	surgeAt func(ctx context.Context, at routing.Point) decimal.Decimal
	// profiles resolves the pricing profile of a quote; tz times its charges.
	profiles Profiles
	tz       func(country string) *time.Location
}

// SetSurge connects the busy-area multiplier at the pickup (nil = no surge).
func (s *Service) SetSurge(f func(ctx context.Context, at routing.Point) decimal.Decimal) {
	s.surgeAt = f
}

// New wires the pricing service.
// SetAreas plugs in the operating-areas resolver; without it every quote is priced at the home country's defaults.
func (s *Service) SetAreas(r AreaResolver) { s.areas = r }

// FeePolicyFor returns the rules in force for a country and optional area: the
// region's profile when it has one, else the country default (in memory).
func (s *Service) FeePolicyFor(ctx context.Context, country, areaID string) (FeePolicy, error) {
	if country == "" {
		country = s.store.homeCountry
	}
	if s.profiles == nil {
		return FeePolicy{}, ErrNoTariff
	}
	res, err := s.profiles.Resolve(ctx, profiles.Input{Country: country, AreaID: areaID})
	if err != nil {
		return FeePolicy{}, ErrNoTariff
	}
	return feePolicyOf(country, areaID, res.Terms, res.Profile.Version), nil
}

func New(pool *pgxpool.Pool, brandID, homeCountry string, cat *catalog.Catalog, router routing.Provider) *Service {
	st := store{pool: pool, brandID: brandID, homeCountry: homeCountry}
	return &Service{
		store:   st,
		catalog: cat,
		routing: router,
		cache:   cache.New("pricing", st, func(ctx context.Context) (tariffs, error) { return st.load(ctx) }, 5*time.Second),
		schemas: newSchemaCache(),
		now:     time.Now,
	}
}

// Engines lists the pricing modules an admin can choose from.
func (s *Service) Engines() []engine.Description { return engine.Describe() }

// FeePolicy returns the home country's default rules (debt limits, payout rules).
func (s *Service) FeePolicy(ctx context.Context) (FeePolicy, error) {
	return s.FeePolicyFor(ctx, s.store.homeCountry, "")
}

// Seed loads seed/pricing/*.json (tariffs + fee policy).
func (s *Service) Seed(ctx context.Context, dir string) (int, error) {
	return s.store.seed(ctx, dir, s.seedPolicy)
}

// Quote prices every eligible vehicle class of a service for a route.
//
//  1. the service must exist; its request schema validates the attributes
//  2. the route comes from the routing provider (dropoff required unless the
//     lifecycle is PICKUP_ONLY)
//  3. only the active vehicle classes are priced
//  4. each remaining class is priced with the pure engine; classes without a
//     tariff are returned as unavailable rather than failing the whole quote
//  5. the quote is persisted with its pricing version and expiry
func (s *Service) Quote(ctx context.Context, req Request) (*Quote, error) {
	if s.profiles == nil {
		return nil, ErrNoTariff
	}
	q, err := s.price(ctx, req, s.profiles.Resolve, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.store.saveQuote(ctx, req, q, req.Attributes); err != nil {
		return nil, fmt.Errorf("save quote: %w", err)
	}
	return q, nil
}

// resolver picks the pricing profile for a quote (live, or a preview).
type resolver func(ctx context.Context, in profiles.Input) (*profiles.Resolution, error)

// price is steps 1–4 of Quote at time `at`, with profiles resolved by `resolve`; nothing is saved.
func (s *Service) price(ctx context.Context, req Request, resolve resolver, at time.Time) (*Quote, error) {
	svc, err := s.catalog.Find(ctx, req.ServiceKey)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, ErrUnknownService
	}
	if err := s.validateAttributes(svc, req.Attributes); err != nil {
		return nil, err
	}
	profile := lifecycleProfile(svc)
	if req.Dropoff == nil && profile != "pickup_only" {
		return nil, ErrDropoffRequired
	}

	if len(req.Waypoints) > routing.MaxWaypoints || (len(req.Waypoints) > 0 && req.Dropoff == nil) {
		return nil, ErrTooManyStops
	}
	route := routing.Route{Provider: "none"}
	if req.Dropoff != nil {
		points := append(append([]routing.Point{req.Pickup}, req.Waypoints...), *req.Dropoff)
		route, err = routing.Via(ctx, s.routing, points)
		route.Steps = nil // navigation detail; quotes store distance, time and the line only
		if err != nil {
			slog.Warn("routing failed", "err", err)
			return nil, ErrRoutingFailed
		}
	}

	tf, version, err := s.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	// Where the pickup is decides the country, the area and therefore the prices and currency.
	area, err := s.resolveArea(ctx, req)
	if err != nil {
		return nil, err
	}
	in := s.profileInput(ctx, req, area, route)
	in.At = at
	res, err := resolve(ctx, in)
	if errors.Is(err, profiles.ErrNoDefault) {
		return nil, ErrNoTariff
	}
	if err != nil {
		return nil, err
	}
	policy := feePolicyOf(area.Ref.Country, area.Ref.ID, res.Terms, res.Profile.Version)
	local := s.localTime(area.Ref.Country, at)
	pinned := map[string]profiles.QuoteClassPinned{}
	// finish adds the profile's charges, rounding and waiting/cancellation terms, and pins the class.
	finish := func(o Option, t engine.Tariff) Option {
		o, applied, added := withCharges(o, res, svc.Key, route.DistanceM, local)
		o = withWaiting(o, t, policy, added, policy.BookingFee.Add(carvedOf(applied)))
		if o.Available {
			pinned[o.VehicleClass] = profiles.QuoteClassPinned{Terms: res.Terms, Charges: applied}
		}
		return o
	}
	surge := decimal.NewFromInt(1)
	if s.surgeAt != nil && area.SurgeEnabled {
		surge = s.surgeAt(ctx, req.Pickup) // locked on the quote; the area and each tariff cap what they accept
		if area.SurgeMax != nil && surge.GreaterThan(*area.SurgeMax) {
			surge = *area.SurgeMax
		}
	}
	q := &Quote{ServiceKey: svc.Key, Route: route, Surge: surge, ExpiresAt: s.now().Add(QuoteTTL), PricingVersion: version,
		Country: area.Ref.Country, Area: area.Ref}
	for _, cls := range activeClasses(svc) {
		t, ok := tf.lookup(area.Ref.Country, area.Ref.ID, svc.Key, cls.Key)
		// Precedence: fixed route beats the metered tariff (manual prices are set after booking).
		if req.Dropoff != nil && len(req.Waypoints) == 0 { // a stop makes it a different journey
			if fr, dir := matchFixedRoute(routesInScope(tf.Routes, area.Ref), svc.Key, req.Pickup, *req.Dropoff, cls.Key, at); fr != nil {
				q.Options = append(q.Options, finish(fixedRouteOption(fr, dir, t, cls.Key, surge, policy.BookingFee), t))
				continue
			}
		}
		eng := engine.Get(t.Engine)
		if !ok || eng == nil || (t.Engine == engine.DefaultKey && len(t.Distance) == 0) {
			q.Options = append(q.Options, Option{VehicleClass: cls.Key, Available: false, Reason: "PRICING_NOT_CONFIGURED"})
			continue
		}
		in := engine.QuoteInput{
			DistanceM:  decimal.NewFromInt(int64(route.DistanceM)),
			DurationS:  decimal.NewFromInt(int64(route.DurationS)),
			Surge:      surge,
			BookingFee: policy.BookingFee,
		}
		total, lines := eng.Quote(t, in)
		q.Options = append(q.Options, finish(Option{VehicleClass: cls.Key, Fare: total, Currency: t.Currency, Lines: lines, Available: true, Source: "METERED"}, t))
	}
	if len(q.Options) == 0 {
		return nil, ErrNoTariff
	}
	q.pricing = quotePricing(res, pinned)
	return q, nil
}

// profileInput is where the trip goes, for profile resolution: the pickup's
// area, the drop-off and its area, and the quoted road line (geofences and
// route-crossing charges are found from these in memory).
func (s *Service) profileInput(ctx context.Context, req Request, area AreaRule, route routing.Route) profiles.Input {
	in := profiles.Input{Country: area.Ref.Country, AreaID: area.Ref.ID, AreaName: area.Ref.Name, At: s.now(),
		Pickup: &geofences.Point{Lat: req.Pickup.Lat, Lng: req.Pickup.Lng}}
	if req.Dropoff == nil {
		return in
	}
	in.Dropoff = &geofences.Point{Lat: req.Dropoff.Lat, Lng: req.Dropoff.Lng}
	if s.areas != nil {
		if ref, ok, err := s.areas.AreaOf(ctx, *req.Dropoff); err == nil && ok {
			in.DropAreaID, in.DropAreaName = ref.ID, ref.Name
		}
	}
	for _, p := range routing.DecodePolyline(route.Polyline) {
		in.Route = append(in.Route, geofences.Point{Lat: p.Lat, Lng: p.Lng})
	}
	return in
}

// fixedRouteOption prices a class on a fixed route: price (× surge when the
// route allows it) + booking fee.
func fixedRouteOption(fr *FixedRoute, dir Direction, t engine.Tariff, class string, surge decimal.Decimal, bookingFee decimal.Decimal) Option {
	multiplier := decimal.NewFromInt(1)
	if fr.SurgeApplies && surge.IsPositive() {
		multiplier = surge
	}
	fare := fr.PriceFor(dir).Mul(multiplier).Round(2)
	direction := "forward"
	if dir == Reverse {
		direction = "reverse"
	}
	lines := []engine.Line{{Code: LineFixedRoute, Amount: fare, Meta: map[string]any{"route": fr.Name, "direction": direction, "multiplier": multiplier.String()}}}
	if bookingFee.IsPositive() {
		lines = append(lines, engine.Line{Code: engine.LineBookingFee, Amount: bookingFee})
		fare = fare.Add(bookingFee)
	}
	currency := fr.Currency
	if currency == "" {
		currency = t.Currency
	}
	return Option{VehicleClass: class, Fare: fare.Round(2), Currency: currency, Lines: lines, Available: true,
		Source: "FIXED_ROUTE", RouteName: fr.Name, FixedRouteID: fr.ID}
}

// validateAttributes checks the request against the service's request schema.
func (s *Service) validateAttributes(svc *catalog.Service, raw json.RawMessage) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	schema, err := s.schemas.get(svc.Key, svc.RequestSchema)
	if err != nil {
		return fmt.Errorf("request schema for %s: %w", svc.Key, err)
	}
	var value any
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&value); err != nil {
		return ErrAttributesInvalid.WithDetails(map[string]any{"reason": "attributes must be a JSON object"})
	}
	if err := schema.Validate(value); err != nil {
		return ErrAttributesInvalid.WithDetails(map[string]any{"fields": schemaErrors(err)})
	}
	return nil
}

// activeClasses are the vehicle classes that can be booked now.
func activeClasses(svc *catalog.Service) []catalog.VehicleClass {
	var out []catalog.VehicleClass
	for _, c := range svc.VehicleClasses {
		if c.Status == "ACTIVE" {
			out = append(out, c)
		}
	}
	return out
}

func lifecycleProfile(svc *catalog.Service) string {
	var lc struct {
		Profile string `json:"profile"`
	}
	_ = json.Unmarshal(svc.Lifecycle, &lc)
	return lc.Profile
}

func schemaErrors(err error) []map[string]string {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []map[string]string{{"path": "", "code": "INVALID"}}
	}
	var out []map[string]string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			path := "/"
			if len(e.InstanceLocation) > 0 {
				path = "/" + joinPath(e.InstanceLocation)
			}
			out = append(out, map[string]string{"path": path, "code": "INVALID", "message": e.ErrorKind.KeywordPath()[0]})
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

func joinPath(parts []string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += "/"
		}
		s += p
	}
	return s
}

// schemaCache compiles each service's request schema once per catalogue change.
// Shared by every request goroutine, so all access goes through mu.
type schemaCache struct {
	mu    sync.RWMutex
	byKey map[string]compiled
}

type compiled struct {
	raw    string
	schema *jsonschema.Schema
}

func newSchemaCache() *schemaCache { return &schemaCache{byKey: map[string]compiled{}} }

func (c *schemaCache) get(key string, raw json.RawMessage) (*jsonschema.Schema, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{"type":"object"}`)
	}
	c.mu.RLock()
	entry, ok := c.byKey[key]
	c.mu.RUnlock()
	if ok && entry.raw == string(raw) {
		return entry.schema, nil
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertVocabs() // x-ui and other unknown keywords are ignored, not errors
	url := "mem://services/" + key + ".json"
	if err := compiler.AddResource(url, doc); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.byKey[key] = compiled{raw: string(raw), schema: schema}
	c.mu.Unlock()
	return schema, nil
}

// withWaiting rounds the fare and adds the pickup waiting terms (free seconds,
// per-minute rate and a cap on the fare without extra charges) and the
// cancellation terms from the class tariff and the profile's rules.
func withWaiting(o Option, t engine.Tariff, p FeePolicy, charges, floor decimal.Decimal) Option {
	o = roundedAtLeast(o, p.FareRounding, floor)
	o.WaitingGraceS = t.LoadingGraceS // the tariff's grace at the first stop is the pickup's free waiting
	o.WaitingRatePerMin = t.WaitingRatePerMin
	if p.WaitingCapRate.IsPositive() {
		o.WaitingCap = o.Fare.Sub(charges).Mul(p.WaitingCapRate).Round(0)
	}
	o.CancellationFee, o.CancelFreeS, o.NoShowAfterS, o.CancelDriverShare = t.CancellationFee, p.CancelFreeS, p.NoShowAfterS, p.CancelDriverShare
	return o
}

// LineRounding is the receipt line that brings a fare to a round amount.
const LineRounding = "ROUNDING"

// roundedAtLeast rounds like rounded, but never below floor (the booking fee
// plus the charges carved out of the split), so the split can always be made.
func roundedAtLeast(o Option, step, floor decimal.Decimal) Option {
	r := rounded(o, step)
	if !r.Available || !step.IsPositive() || !r.Fare.LessThan(floor) {
		return r
	}
	up := o.Fare.Div(step).Ceil().Mul(step)
	if diff := up.Sub(o.Fare); !diff.IsZero() {
		o.Lines = append(o.Lines, engine.Line{Code: LineRounding, Amount: diff})
	}
	o.Fare = up
	return o
}

// rounded brings the option's fare to the nearest `step` (half up) and records
// the difference as a ROUNDING line so the lines still add up to the fare.
func rounded(o Option, step decimal.Decimal) Option {
	if !step.IsPositive() || !o.Available {
		return o
	}
	r := o.Fare.Div(step).Round(0).Mul(step)
	if diff := r.Sub(o.Fare); !diff.IsZero() {
		o.Lines = append(o.Lines, engine.Line{Code: LineRounding, Amount: diff})
		o.Fare = r
	}
	return o
}

// resolveArea finds the pickup's area and refuses drop-offs in another country.
// Without a resolver (tests, single-country brands before areas are marked) the
// home country's defaults apply.
func (s *Service) resolveArea(ctx context.Context, req Request) (AreaRule, error) {
	if s.areas == nil {
		return AreaRule{Ref: AreaRef{Country: s.store.homeCountry}, SurgeEnabled: true}, nil
	}
	rule, err := s.areas.Serviceable(ctx, req.Pickup)
	if err != nil {
		return AreaRule{}, err
	}
	points := append([]routing.Point{}, req.Waypoints...)
	if req.Dropoff != nil {
		points = append(points, *req.Dropoff)
	}
	for _, p := range points {
		if ref, ok, err := s.areas.AreaOf(ctx, p); err != nil {
			return AreaRule{}, err
		} else if ok && ref.Country != rule.Ref.Country {
			return AreaRule{}, ErrCrossBorder
		}
	}
	return rule, nil
}

// routesInScope keeps the fixed routes of the pickup's area and its country-wide ones.
func routesInScope(routes []FixedRoute, area AreaRef) []FixedRoute {
	out := make([]FixedRoute, 0, len(routes))
	for _, r := range routes {
		if r.Country == area.Country && (r.AreaID == "" || r.AreaID == area.ID) {
			out = append(out, r)
		}
	}
	return out
}
