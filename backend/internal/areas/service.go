package areas

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/uber/h3-go/v4"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)

// Service owns countries and areas and answers where a point is.
type Service struct {
	store store
	brand *brand.Brand
	index *cache.Versioned[*Index]
	audit Auditor
}

// New builds the service; the index reloads when the "areas" config version moves.
func New(pool *pgxpool.Pool, brandID string, b *brand.Brand, audit Auditor) *Service {
	s := &Service{store: store{pool: pool, brandID: brandID}, brand: b, audit: audit}
	s.index = cache.New("areas", s.store, s.load, CacheTTL)
	return s
}

func (s *Service) load(ctx context.Context) (*Index, error) {
	areas, cells, err := s.store.loadAll(ctx)
	if err != nil {
		return nil, err
	}
	active := map[string]bool{}
	for _, c := range s.brand.Countries {
		active[c.Code] = c.Status == brand.CountryActive
	}
	return Build(areas, cells, active), nil
}

// Index returns the current cell index.
func (s *Service) Index(ctx context.Context) (*Index, error) {
	ix, _, err := s.index.Get(ctx)
	return ix, err
}

// Resolve returns the serviceable area a point is in, or ErrOutsideServiceArea.
func (s *Service) Resolve(ctx context.Context, p routing.Point) (AreaRef, error) {
	ix, err := s.Index(ctx)
	if err != nil {
		return AreaRef{}, err
	}
	ref, ok := ix.Serviceable(p)
	if !ok {
		return AreaRef{}, ErrOutsideServiceArea
	}
	return ref, nil
}

// Countries lists the brand's countries as stored (with area counts).
func (s *Service) Countries(ctx context.Context) ([]Country, error) { return s.store.countries(ctx) }

// Country returns one brand country or ErrCountryUnknown.
func (s *Service) Country(code string) (*brand.Country, error) {
	c := s.brand.CountryByCode(strings.ToUpper(code))
	if c == nil {
		return nil, ErrCountryUnknown
	}
	return c, nil
}

// List returns the non-archived areas of a country.
func (s *Service) List(ctx context.Context, country string) ([]*Area, error) {
	if _, err := s.Country(country); err != nil {
		return nil, err
	}
	ix, err := s.Index(ctx)
	if err != nil {
		return nil, err
	}
	out := []*Area{}
	for _, a := range ix.areas {
		if a.Country == strings.ToUpper(country) {
			out = append(out, a)
		}
	}
	sortAreas(out)
	return out, nil
}

// Get returns one area with its cells (any status).
func (s *Service) Get(ctx context.Context, id string) (*Area, error) {
	a, cells, err := s.store.get(ctx, s.store.pool, id)
	if err != nil {
		return nil, err
	}
	a.CellCount, a.Cells, a.BBox = len(cells), Compact(cells), BBox(cells)
	return a, nil
}

// CreateInput is what staff send to create an area.
type CreateInput struct {
	Country      string           `json:"country"`
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	SurgeEnabled *bool            `json:"surge_enabled"`
	SurgeMax     *decimal.Decimal `json:"surge_max"`
	Reason       string           `json:"reason"`
}

// Create adds a DRAFT area (audited).
func (s *Service) Create(ctx context.Context, staffID string, in CreateInput) (*Area, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrReasonRequired
	}
	if _, err := s.Country(in.Country); err != nil {
		return nil, err
	}
	if !codeRe.MatchString(in.Code) || strings.TrimSpace(in.Name) == "" || !validSurgeMax(in.SurgeMax) {
		return nil, ErrCellInvalid.WithDetails(map[string]any{"fields": []string{"code", "name", "surge_max"}})
	}
	a := Area{Country: strings.ToUpper(in.Country), Code: in.Code, Name: strings.TrimSpace(in.Name), SurgeEnabled: in.SurgeEnabled == nil || *in.SurgeEnabled, SurgeMax: in.SurgeMax}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		id, err := s.store.insert(ctx, tx, a)
		if err != nil {
			return err
		}
		a.ID = id
		if err := s.store.bump(ctx, tx); err != nil {
			return err
		}
		return s.audit(ctx, tx, s.store.brandID, staffID, "areas.create", "area:"+id, in.Reason, a)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, a.ID)
}

// UpdateInput changes an area's name, status or surge rules.
type UpdateInput struct {
	Name         *string          `json:"name"`
	Status       *string          `json:"status"`
	SurgeEnabled *bool            `json:"surge_enabled"`
	SurgeMax     *decimal.Decimal `json:"surge_max"`
	Reason       string           `json:"reason"`
}

// Update applies the lifecycle rules and saves (audited).
func (s *Service) Update(ctx context.Context, staffID, id string, in UpdateInput) (*Area, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrReasonRequired
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		a, _, err := s.store.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
			a.Name = strings.TrimSpace(*in.Name)
		}
		if in.SurgeEnabled != nil {
			a.SurgeEnabled = *in.SurgeEnabled
		}
		if in.SurgeMax != nil || (in.SurgeEnabled != nil && !*in.SurgeEnabled) {
			if !validSurgeMax(in.SurgeMax) {
				return ErrCellInvalid.WithDetails(map[string]any{"fields": []string{"surge_max"}})
			}
			a.SurgeMax = in.SurgeMax
		}
		if in.Status != nil && *in.Status != a.Status {
			if err := s.checkTransition(ctx, tx, a, *in.Status); err != nil {
				return err
			}
			a.Status = *in.Status
			if a.Status == StatusArchived {
				if err := s.store.replaceCells(ctx, tx, a.ID, nil); err != nil {
					return err
				}
			}
		}
		if err := s.store.update(ctx, tx, a); err != nil {
			return err
		}
		if err := s.store.bump(ctx, tx); err != nil {
			return err
		}
		return s.audit(ctx, tx, s.store.brandID, staffID, "areas.update", "area:"+id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// checkTransition enforces the lifecycle rules for a status change.
func (s *Service) checkTransition(ctx context.Context, tx pgx.Tx, a *Area, to string) error {
	switch to {
	case StatusActive:
		st, err := s.store.countryStatus(ctx, tx, a.Country)
		if err != nil {
			return err
		}
		if st != brand.CountryActive {
			return ErrCountryNotActive
		}
		missing, err := s.store.readiness(ctx, tx, a)
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			return ErrAreaNotReady.WithDetails(map[string]any{"missing": missing})
		}
	case StatusDraft, StatusArchived:
		drivers, trips, err := s.store.inUse(ctx, tx, a.ID)
		if err != nil {
			return err
		}
		if trips > 0 {
			return ErrAreaHasActiveTrips
		}
		if to == StatusArchived && drivers > 0 {
			return ErrAreaHasDrivers.WithDetails(map[string]any{"count": drivers})
		}
	default:
		return ErrCellInvalid.WithDetails(map[string]any{"fields": []string{"status"}})
	}
	return nil
}

// SaveCells replaces an area's cells (audited). Overlap with another area is refused
// with the conflicting cells, under a brand-wide lock; the primary key is the final guard.
func (s *Service) SaveCells(ctx context.Context, staffID, id string, cells []string, reason string) (*Area, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, ErrReasonRequired
	}
	fine, err := Expand(cells)
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('areas:' || $1))`, s.store.brandID); err != nil {
			return err
		}
		a, _, err := s.store.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if a.Status == StatusArchived {
			return ErrAreaArchived
		}
		ix, err := s.Index(ctx)
		if err != nil {
			return err
		}
		if conflicts, owners := ix.Overlaps(id, fine); len(conflicts) > 0 {
			return ErrCellsOverlap.WithDetails(overlapDetails(conflicts, owners))
		}
		if err := s.store.replaceCells(ctx, tx, id, fine); err != nil {
			return err
		}
		if err := s.store.bump(ctx, tx); err != nil {
			return err
		}
		return s.audit(ctx, tx, s.store.brandID, staffID, "areas.cells", "area:"+id, reason, map[string]any{"cells": len(fine)})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func overlapDetails(conflicts []h3.Cell, owners map[string]AreaRef) map[string]any {
	cs := make([]string, 0, min(len(conflicts), OverlapDetailLimit))
	for i, c := range conflicts {
		if i == OverlapDetailLimit {
			break
		}
		cs = append(cs, c.String())
	}
	refs := make([]AreaRef, 0, len(owners))
	for _, r := range owners {
		refs = append(refs, r)
	}
	return map[string]any{"cells": cs, "areas": refs}
}

// DriverAreas lists a driver's areas and the primary one.
func (s *Service) DriverAreas(ctx context.Context, driverID string) ([]AreaRef, string, error) {
	return s.store.driverAreas(ctx, driverID)
}

// SetDriverAreas replaces a driver's areas: 1 to MaxDriverAreas, all in one country,
// with one primary. actor "" = the driver at sign-up; staff edits are audited.
func (s *Service) SetDriverAreas(ctx context.Context, actorID, driverID string, areaIDs []string, primary, reason string) ([]AreaRef, error) {
	if len(areaIDs) == 0 {
		return nil, ErrAreaRequired
	}
	if len(areaIDs) > MaxDriverAreas {
		return nil, ErrCellInvalid.WithDetails(map[string]any{"max": MaxDriverAreas})
	}
	if actorID != "" && strings.TrimSpace(reason) == "" {
		return nil, ErrReasonRequired
	}
	ix, err := s.Index(ctx)
	if err != nil {
		return nil, err
	}
	country := ""
	hasPrimary := false
	for _, id := range areaIDs {
		a, ok := ix.Area(id)
		if !ok {
			return nil, ErrAreaNotFound
		}
		if country != "" && a.Country != country {
			return nil, ErrDriverAreasCountry
		}
		country = a.Country
		if id == primary {
			hasPrimary = true
		}
	}
	if primary == "" {
		primary, hasPrimary = areaIDs[0], true
	}
	if !hasPrimary {
		return nil, ErrPrimaryNotInAreas
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.setDriverAreas(ctx, tx, driverID, country, areaIDs, primary); err != nil {
			return err
		}
		if actorID == "" {
			return nil
		}
		return s.audit(ctx, tx, s.store.brandID, actorID, "driver.areas", "driver:"+driverID, reason, map[string]any{"areas": areaIDs, "primary": primary})
	})
	if err != nil {
		return nil, err
	}
	refs, _, err := s.store.driverAreas(ctx, driverID)
	return refs, err
}

// seedArea mirrors seed/areas/<CC>.json.
type seedFile struct {
	Country string `json:"country"`
	Areas   []struct {
		Code    string       `json:"code"`
		Name    string       `json:"name"`
		Status  string       `json:"status"`
		Polygon [][2]float64 `json:"polygon"`
	} `json:"areas"`
}

// Seed writes the brand's countries and the launch areas. Cells are written only
// for areas that have none, so staff edits are never overwritten.
func (s *Service) Seed(ctx context.Context, dir string) (int, error) {
	for _, c := range s.brand.Countries {
		if err := s.store.upsertCountry(ctx, Country{Code: c.Code, Name: c.Name, Currency: c.Currency, Timezone: c.Timezone, AreaTerm: c.AreaTerm,
			Regulator: c.Regulator, PaymentGateway: c.PaymentGateway, EmergencyPhone: c.EmergencyPhone, Map: MapView(c.Map), Status: c.Status,
			LicencePattern: c.LicencePattern, LicenceClasses: c.LicenceClasses}); err != nil {
			return 0, err
		}
	}
	n := 0
	for _, c := range s.brand.Countries {
		raw, err := os.ReadFile(filepath.Join(dir, c.Code+".json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return n, err
		}
		var f seedFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return n, fmt.Errorf("areas seed %s: %w", c.Code, err)
		}
		for _, sa := range f.Areas {
			if err := s.seedOne(ctx, c.Code, sa.Code, sa.Name, sa.Status, sa.Polygon); err != nil {
				return n, fmt.Errorf("area %s/%s: %w", c.Code, sa.Code, err)
			}
			n++
		}
	}
	if err := s.backfillDrivers(ctx); err != nil {
		return n, err
	}
	return n, pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error { return s.store.bump(ctx, tx) })
}

// backfillDrivers registers approved drivers who have no area yet in the home
// country's first ACTIVE area (idempotent), so existing drivers keep working.
func (s *Service) backfillDrivers(ctx context.Context) error {
	_, err := s.store.pool.Exec(ctx, `
		WITH home AS (
		  SELECT a.id FROM areas a WHERE a.brand_id=$1 AND a.country_code=$2 AND a.status='ACTIVE' ORDER BY a.created_at LIMIT 1)
		INSERT INTO driver_areas (driver_id, area_id, is_primary)
		SELECT d.user_id, home.id, true FROM driver_profiles d, home
		WHERE d.brand_id=$1 AND d.status='APPROVED' AND NOT EXISTS (SELECT 1 FROM driver_areas da WHERE da.driver_id=d.user_id)
		ON CONFLICT DO NOTHING`, s.store.brandID, s.brand.Country)
	return err
}

func (s *Service) seedOne(ctx context.Context, country, code, name, status string, polygon [][2]float64) error {
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		var id string
		var existing int
		err := tx.QueryRow(ctx, `INSERT INTO areas (brand_id, country_code, code, name, status) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (brand_id, country_code, code) DO UPDATE SET updated_at = areas.updated_at
			RETURNING id, (SELECT count(*) FROM area_cells WHERE area_id = areas.id)`, s.store.brandID, country, code, name, status).Scan(&id, &existing)
		if err != nil || existing > 0 {
			return err
		}
		cells, err := PolygonToCells(polygon)
		if err != nil {
			return err
		}
		return s.store.replaceCells(ctx, tx, id, cells)
	})
}

func validSurgeMax(m *decimal.Decimal) bool {
	return m == nil || (m.GreaterThanOrEqual(decimal.NewFromInt(1)) && m.LessThanOrEqual(decimal.NewFromInt(2)))
}

func sortAreas(a []*Area) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j].Name < a[j-1].Name; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
