package geofences

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
)

// indexCheckTTL bounds how long an instance prices with an old geofence index after a change elsewhere.
const indexCheckTTL = 5 * time.Second

var codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,47}$`)

// Service is the geofence use cases.
type Service struct {
	store    store
	areas    Areas
	profiles Profiles
	index    *cache.Versioned[*Index]
}

// New wires the service; areas and profiles may be set later (SetPorts).
func New(pool *pgxpool.Pool, brandID string) *Service {
	s := &Service{store: store{pool: pool, brandID: brandID}}
	s.index = cache.New("geofences", s.store, s.loadIndex, indexCheckTTL)
	return s
}

// SetPorts connects the areas (country check) and profiles (assignments) modules.
func (s *Service) SetPorts(a Areas, p Profiles) { s.areas, s.profiles = a, p }

func (s *Service) loadIndex(ctx context.Context) (*Index, error) {
	fences, err := s.store.list(ctx, s.store.pool, `g.status='ACTIVE'`)
	if err != nil {
		return nil, err
	}
	return NewIndex(fences)
}

// Contains returns the active geofences at a point (in memory).
func (s *Service) Contains(ctx context.Context, p Point) ([]Hit, error) {
	ix, _, err := s.index.Get(ctx)
	return ix.Contains(p), err
}

// Crossed returns the active zones a route passes through (in memory).
func (s *Service) Crossed(ctx context.Context, route []Point) ([]Hit, error) {
	ix, _, err := s.index.Get(ctx)
	return ix.Crossed(route), err
}

// List returns a country's geofences (kind and status optional), with the profile open on each.
func (s *Service) List(ctx context.Context, country, kind, status string) ([]Geofence, error) {
	fences, err := s.store.list(ctx, s.store.pool, `($2='' OR g.country_code=$2) AND ($3='' OR g.kind=$3) AND ($4='' OR g.status=$4)`,
		strings.ToUpper(country), strings.ToUpper(kind), strings.ToUpper(status))
	if err != nil {
		return nil, err
	}
	return fences, s.withProfiles(ctx, fences)
}

func (s *Service) withProfiles(ctx context.Context, fences []Geofence) error {
	if s.profiles == nil || len(fences) == 0 {
		return nil
	}
	ids := make([]string, len(fences))
	for i, f := range fences {
		ids[i] = f.ID
	}
	open, err := s.profiles.OpenOnScopes(ctx, ids)
	for i := range fences {
		if a, ok := open[fences[i].ID]; ok {
			fences[i].Profile = &a
		}
	}
	return err
}

// Get returns one geofence with its versions.
func (s *Service) Get(ctx context.Context, id string) (*Geofence, error) {
	f, err := s.store.get(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	one := []Geofence{f}
	if err := s.withProfiles(ctx, one); err != nil {
		return nil, err
	}
	f = one[0]
	f.Versions, err = s.store.versions(ctx, id)
	return &f, err
}

// Info is a geofence's country, name and kind (profile assignments check it).
func (s *Service) Info(ctx context.Context, id string) (country, name, kind string, ok bool, err error) {
	f, err := s.store.get(ctx, s.store.pool, id, false)
	if err == ErrNotFound {
		return "", "", "", false, nil
	}
	return f.Country, f.Name, f.Kind, err == nil && f.Status == StatusActive, err
}

// CreateInput is a new geofence.
type CreateInput struct {
	Country string `json:"country"`
	Kind    string `json:"kind"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	ShapeInput
	Reason string `json:"reason"`
}

// Create validates and saves a new geofence (version 1), audited.
func (s *Service) Create(ctx context.Context, staffID string, in CreateInput) (*Geofence, error) {
	in.Country, in.Kind, in.Code, in.Name = strings.ToUpper(strings.TrimSpace(in.Country)), strings.ToUpper(in.Kind), strings.TrimSpace(in.Code), strings.TrimSpace(in.Name)
	if err := checkNaming(in); err != nil {
		return nil, err
	}
	var id string
	err := s.write(ctx, in.Reason, func(tx pgx.Tx) error {
		sh, err := s.shape(ctx, tx, in.Country, in.Kind, in.ShapeInput)
		if err != nil {
			return err
		}
		f := Geofence{Country: in.Country, Kind: in.Kind, Code: in.Code, Name: in.Name}
		if id, err = s.store.insert(ctx, tx, staffID, f, sh, in.Reason); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "geofence.create", "geofence:"+id, in.Reason, map[string]any{"code": in.Code, "kind": in.Kind})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Reshape saves a new shape as the next version, audited.
func (s *Service) Reshape(ctx context.Context, staffID, id string, in ShapeInput, reason string) (*Geofence, error) {
	err := s.write(ctx, reason, func(tx pgx.Tx) error {
		f, err := s.store.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if f.Status == StatusArchived {
			return ErrArchived
		}
		sh, err := s.shape(ctx, tx, f.Country, f.Kind, in)
		if err != nil {
			return err
		}
		if err := s.store.reshape(ctx, tx, staffID, f, sh, reason); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "geofence.reshape", "geofence:"+id, reason, map[string]any{"version": f.Version + 1})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// UpdateInput renames a geofence or changes its status.
type UpdateInput struct {
	Name   *string `json:"name"`
	Status *string `json:"status"`
	Reason string  `json:"reason"`
}

// Update renames or switches a geofence (not off or archived while a profile is applied to it).
func (s *Service) Update(ctx context.Context, staffID, id string, in UpdateInput) (*Geofence, error) {
	err := s.write(ctx, in.Reason, func(tx pgx.Tx) error {
		f, err := s.store.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		name, status := f.Name, f.Status
		if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
			name = strings.TrimSpace(*in.Name)
		}
		if in.Status != nil {
			status = strings.ToUpper(*in.Status)
		}
		if err := s.checkStatus(ctx, f, status); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE geofences SET name=$2, status=$3, updated_at=now() WHERE id=$1`, id, name, status); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "geofence.update", "geofence:"+id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) checkStatus(ctx context.Context, f Geofence, status string) error {
	if status != StatusActive && status != StatusInactive && status != StatusArchived {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "status", "code": "NOT_ALLOWED"}}})
	}
	if status == StatusActive || s.profiles == nil {
		return nil
	}
	open, err := s.profiles.OpenOnScopes(ctx, []string{f.ID})
	if err != nil {
		return err
	}
	if a, ok := open[f.ID]; ok {
		return ErrInUse.WithDetails(map[string]any{"profile": a})
	}
	return nil
}

// Overlaps lists active geofences of a country whose shapes meet (optionally only those touching one geofence).
func (s *Service) Overlaps(ctx context.Context, country, onlyID string) ([]Overlap, error) {
	return s.store.overlaps(ctx, strings.ToUpper(country), onlyID)
}

// write runs a change with the geofences version bump in one transaction.
func (s *Service) write(ctx context.Context, reason string, fn func(tx pgx.Tx) error) error {
	if strings.TrimSpace(reason) == "" {
		return ErrReason
	}
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := fn(tx); err != nil {
			return err
		}
		return bump(ctx, tx)
	})
}

// shape validates a polygon (districts, zones) or a centre and radius (places) and measures it.
func (s *Service) shape(ctx context.Context, tx pgx.Tx, country, kind string, in ShapeInput) (shapeArgs, error) {
	if kind == KindPlace {
		return s.placeShape(ctx, country, in)
	}
	if len(in.Geometry) == 0 {
		return shapeArgs{}, invalid("WRONG_TYPE", "a district or zone needs a polygon")
	}
	g, err := ParseGeometry(in.Geometry)
	if err != nil {
		return shapeArgs{}, err
	}
	if g.Vertices() > MaxVertices {
		return shapeArgs{}, invalid("TOO_MANY_VERTICES", fmt.Sprintf("%d points; at most %d", g.Vertices(), MaxVertices))
	}
	wkt := g.WKT()
	m, err := s.store.measure(ctx, tx, wkt)
	if err != nil {
		return shapeArgs{}, err
	}
	if err := checkMeasured(kind, m); err != nil {
		return shapeArgs{}, err
	}
	if err := s.checkCountry(ctx, country, m.centroid); err != nil {
		return shapeArgs{}, err
	}
	return shapeArgs{wkt: &wkt, areaM2: m.areaM2, vertices: m.vertices}, nil
}

func checkMeasured(kind string, m measured) error {
	if !m.valid {
		return invalid("SELF_INTERSECTION", m.reason)
	}
	if m.areaM2 < MinAreaM2 {
		return invalid("TOO_SMALL", fmt.Sprintf("%.0f m²", m.areaM2))
	}
	if limit := MaxAreaKm2[kind] * 1e6; m.areaM2 > limit {
		return invalid("TOO_LARGE", fmt.Sprintf("%.0f km²; at most %.0f km²", m.areaM2/1e6, MaxAreaKm2[kind]))
	}
	return nil
}

func (s *Service) placeShape(ctx context.Context, country string, in ShapeInput) (shapeArgs, error) {
	if in.Center == nil || in.Center.Lat < -90 || in.Center.Lat > 90 || in.Center.Lng < -180 || in.Center.Lng > 180 {
		return shapeArgs{}, invalid("WRONG_TYPE", "a place needs a centre")
	}
	if in.RadiusM < PlaceRadiusMinM || in.RadiusM > PlaceRadiusMaxM {
		return shapeArgs{}, invalid("RADIUS_OUT_OF_RANGE", fmt.Sprintf("%d m; %d to %d m", in.RadiusM, PlaceRadiusMinM, PlaceRadiusMaxM))
	}
	if err := s.checkCountry(ctx, country, *in.Center); err != nil {
		return shapeArgs{}, err
	}
	center := fmt.Sprintf("SRID=4326;POINT(%.7f %.7f)", in.Center.Lng, in.Center.Lat)
	r := in.RadiusM
	return shapeArgs{center: &center, radiusM: &r, areaM2: math.Pi * float64(r) * float64(r), vertices: 1}, nil
}

// checkCountry refuses a shape whose middle lies in another country's operating area.
func (s *Service) checkCountry(ctx context.Context, country string, p Point) error {
	if s.areas == nil {
		return nil
	}
	at, ok, err := s.areas.CountryAt(ctx, p)
	if err != nil {
		return err
	}
	if ok && at != country {
		return ErrOtherCountry.WithDetails(map[string]any{"country": at})
	}
	return nil
}

func checkNaming(in CreateInput) error {
	var bad []map[string]string
	if len(in.Country) != 2 {
		bad = append(bad, map[string]string{"path": "country", "code": "FORMAT"})
	}
	if in.Kind != KindDistrict && in.Kind != KindZone && in.Kind != KindPlace {
		bad = append(bad, map[string]string{"path": "kind", "code": "NOT_ALLOWED"})
	}
	if !codePattern.MatchString(in.Code) {
		bad = append(bad, map[string]string{"path": "code", "code": "FORMAT"})
	}
	if in.Name == "" || len([]rune(in.Name)) > 80 {
		bad = append(bad, map[string]string{"path": "name", "code": "LENGTH"})
	}
	if len(bad) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": bad})
	}
	return nil
}
