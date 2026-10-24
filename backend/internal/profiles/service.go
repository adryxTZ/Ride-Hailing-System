package profiles

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
)

// snapshotCheckTTL bounds how long an instance prices with old profiles after a change elsewhere.
const snapshotCheckTTL = 5 * time.Second

// Service is the profiles use cases.
type Service struct {
	store   store
	catalog Catalog
	areas   Areas
	fences  Geofences // nil until SetGeofences
	snap    *cache.Versioned[*Snapshot]
	now     func() time.Time
}

// New wires the service.
func New(pool *pgxpool.Pool, brandID string, catalog Catalog, areas Areas) *Service {
	s := &Service{store: store{pool: pool, brandID: brandID}, catalog: catalog, areas: areas, now: time.Now}
	s.snap = cache.New("pricing", s.store, func(ctx context.Context) (*Snapshot, error) { return s.store.loadSnapshot(ctx, s.now()) }, snapshotCheckTTL)
	return s
}

// SetGeofences connects districts, zones and places.
func (s *Service) SetGeofences(g Geofences) { s.fences = g }

// Snapshot returns the cached snapshot (no database query unless pricing changed).
func (s *Service) Snapshot(ctx context.Context) (*Snapshot, error) {
	snap, _, err := s.snap.Get(ctx)
	return snap, err
}

// Resolve picks the profile that prices a trip (in memory).
func (s *Service) Resolve(ctx context.Context, in Input) (*Resolution, error) {
	snap, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if in.At.IsZero() {
		in.At = s.now()
	}
	if err := s.locate(ctx, &in); err != nil {
		return nil, err
	}
	return Resolve(snap, in)
}

// locate fills the geofences at the pickup and drop-off and the zones the route crosses (in memory).
func (s *Service) locate(ctx context.Context, in *Input) error {
	in.HasDropoff = in.HasDropoff || in.Dropoff != nil
	if s.fences == nil {
		return nil
	}
	ix, err := s.fences.Index(ctx)
	if err != nil {
		return err
	}
	locateIn(ix, in)
	return nil
}

// locateIn fills an input's geofence lists from an index (today's or a past one).
func locateIn(ix *geofences.Index, in *Input) {
	if in.Pickup != nil {
		in.PickupFences = scopesOf(ix.Contains(*in.Pickup), in.Country)
	}
	if in.Dropoff != nil {
		in.DropoffFences = scopesOf(ix.Contains(*in.Dropoff), in.Country)
	}
	if len(in.Route) > 1 {
		in.CrossedZones = scopesOf(ix.Crossed(in.Route), in.Country)
	}
}

// scopesOf keeps the hits in the trip's country as candidate scopes.
func scopesOf(hits []geofences.Hit, country string) []ScopeRef {
	var out []ScopeRef
	for _, h := range hits {
		if h.Country == country {
			out = append(out, ScopeRef{Kind: h.Kind, ID: h.ID, Name: h.Name})
		}
	}
	return out
}

// DefaultTerms returns a country default's rules now (debt limits, payouts, legacy callers).
func (s *Service) DefaultTerms(ctx context.Context, country string) (Effective, Ref, error) {
	snap, err := s.Snapshot(ctx)
	if err != nil {
		return Effective{}, Ref{}, err
	}
	return DefaultTerms(snap, country, s.now())
}

// List returns a country's profiles: the default first, then by name.
func (s *Service) List(ctx context.Context, country string) ([]Profile, error) {
	ps, err := s.store.profiles(ctx, country)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	now := s.now()
	vs, err := s.store.versions(ctx, s.store.pool, `v.profile_id = ANY($4) AND `+liveVersions, now, snapshotHorizon(now), ids)
	if err != nil {
		return nil, err
	}
	open, err := s.store.assignments(ctx, s.store.pool, `a.valid_to IS NULL AND a.profile_id = ANY($2)`, ids)
	if err != nil {
		return nil, err
	}
	for i := range ps {
		s.summarise(&ps[i], vs, open)
	}
	defaultFirst(ps)
	return ps, nil
}

// summarise fills a profile's default flag, current/next version, charge count and open scopes.
func (s *Service) summarise(p *Profile, vs []Version, open []Scope) {
	now := s.now()
	p.Scopes = []Scope{}
	for _, a := range open {
		if a.ProfileID == p.ID {
			p.Scopes = append(p.Scopes, a)
			p.IsDefault = p.IsDefault || a.Kind == ScopeCountry
		}
	}
	for _, v := range vs { // newest effective first
		if v.ProfileID != p.ID {
			continue
		}
		ref := &VersionRef{ID: v.ID, Version: v.Version, EffectiveFrom: v.EffectiveFrom}
		if v.EffectiveFrom.After(now) {
			p.NextVersion = ref
		} else if p.CurrentVersion == nil {
			p.CurrentVersion, p.ChargesCount = ref, len(v.Charges)
		}
	}
}

func defaultFirst(ps []Profile) {
	for i, p := range ps {
		if p.IsDefault && i > 0 {
			copy(ps[1:i+1], ps[0:i])
			ps[0] = p
			return
		}
	}
}

// Get returns one profile with versions, assignments and history.
func (s *Service) Get(ctx context.Context, id string) (*Profile, error) {
	p, err := s.store.profile(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	if p.Versions, err = s.store.versions(ctx, s.store.pool, `v.profile_id = $2`, id); err != nil {
		return nil, err
	}
	if len(p.Versions) > listVersionsLimit {
		p.Versions = p.Versions[:listVersionsLimit]
	}
	if p.Assignments, err = s.store.assignments(ctx, s.store.pool, `a.profile_id = $2`, id); err != nil {
		return nil, err
	}
	var open []Scope
	for _, a := range p.Assignments {
		if a.ValidTo == nil {
			open = append(open, a)
		}
	}
	s.summarise(&p, p.Versions, open)
	if p.Events, err = s.store.events(ctx, id); err != nil {
		return nil, err
	}
	if p.Versions == nil {
		p.Versions = []Version{}
	}
	return &p, nil
}
