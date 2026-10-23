package areas

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/uber/h3-go/v4"
)

const pgUniqueViolation = "23505"

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// Version implements cache.VersionSource for the "areas" scope.
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

// bump tells every process that areas changed.
func (s store) bump(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='areas'`)
	return err
}

// countries lists the brand's countries with their area counts.
func (s store) countries(ctx context.Context) ([]Country, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.code, c.name, c.currency, c.timezone, c.area_term, c.regulator, c.payment_gateway, c.emergency_phone,
		  c.map_lat, c.map_lng, c.map_zoom, c.status,
		  (SELECT count(*) FROM areas a WHERE a.brand_id=c.brand_id AND a.country_code=c.code AND a.status <> 'ARCHIVED')
		FROM countries c WHERE c.brand_id=$1 ORDER BY c.status, c.code`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Country{}
	for rows.Next() {
		var c Country
		var lat, lng decimal.Decimal
		if err := rows.Scan(&c.Code, &c.Name, &c.Currency, &c.Timezone, &c.AreaTerm, &c.Regulator, &c.PaymentGateway, &c.EmergencyPhone,
			&lat, &lng, &c.Map.Zoom, &c.Status, &c.AreaCount); err != nil {
			return nil, err
		}
		c.Map.Lat, _ = lat.Float64()
		c.Map.Lng, _ = lng.Float64()
		out = append(out, c)
	}
	return out, rows.Err()
}

// upsertCountry writes a brand-file country (seed); staff never edit countries.
func (s store) upsertCountry(ctx context.Context, c Country) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO countries (brand_id, code, name, currency, timezone, area_term, regulator, payment_gateway, emergency_phone, map_lat, map_lng, map_zoom, status,
		  licence_pattern, licence_classes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''),$15)
		ON CONFLICT (brand_id, code) DO UPDATE SET name=EXCLUDED.name, currency=EXCLUDED.currency, timezone=EXCLUDED.timezone,
		  area_term=EXCLUDED.area_term, regulator=EXCLUDED.regulator, payment_gateway=EXCLUDED.payment_gateway, emergency_phone=EXCLUDED.emergency_phone,
		  map_lat=EXCLUDED.map_lat, map_lng=EXCLUDED.map_lng, map_zoom=EXCLUDED.map_zoom, status=EXCLUDED.status,
		  licence_pattern=EXCLUDED.licence_pattern, licence_classes=EXCLUDED.licence_classes, updated_at=now()`,
		s.brandID, c.Code, c.Name, c.Currency, c.Timezone, c.AreaTerm, c.Regulator, c.PaymentGateway, c.EmergencyPhone, c.Map.Lat, c.Map.Lng, c.Map.Zoom, c.Status,
		c.LicencePattern, licenceClasses(c.LicenceClasses))
	return err
}

// areaRow is an area without its cells.
func (s store) scanArea(r pgx.Row) (*Area, error) {
	var a Area
	if err := r.Scan(&a.ID, &a.Country, &a.Code, &a.Name, &a.Status, &a.SurgeEnabled, &a.SurgeMax, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

const areaColumns = `id, country_code, code, name, status, surge_enabled, surge_max, updated_at`

// get loads one area (any status) with its cells.
func (s store) get(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, id string) (*Area, []h3.Cell, error) {
	a, err := s.scanArea(q.QueryRow(ctx, `SELECT `+areaColumns+` FROM areas WHERE brand_id=$1 AND id=$2`, s.brandID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrAreaNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	cells, err := s.cellsOf(ctx, q, id)
	return a, cells, err
}

func (s store) cellsOf(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, areaID string) ([]h3.Cell, error) {
	rows, err := q.Query(ctx, `SELECT cell FROM area_cells WHERE area_id=$1`, areaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []h3.Cell
	for rows.Next() {
		var c int64
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, h3.Cell(c))
	}
	return out, rows.Err()
}

// loadAll reads every non-archived area and every cell in two queries (the index source).
func (s store) loadAll(ctx context.Context) ([]*Area, []cellRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+areaColumns+` FROM areas WHERE brand_id=$1 AND status <> 'ARCHIVED' ORDER BY country_code, name`, s.brandID)
	if err != nil {
		return nil, nil, err
	}
	var areas []*Area
	byID := map[string]*Area{}
	for rows.Next() {
		a, err := s.scanArea(rows)
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		areas = append(areas, a)
		byID[a.ID] = a
	}
	rows.Close()
	crows, err := s.pool.Query(ctx, `SELECT c.cell, c.area_id FROM area_cells c JOIN areas a ON a.id=c.area_id WHERE c.brand_id=$1 AND a.status <> 'ARCHIVED'`, s.brandID)
	if err != nil {
		return nil, nil, err
	}
	defer crows.Close()
	var cells []cellRow
	perArea := map[string][]h3.Cell{}
	for crows.Next() {
		var c int64
		var id string
		if err := crows.Scan(&c, &id); err != nil {
			return nil, nil, err
		}
		a := byID[id]
		cells = append(cells, cellRow{Cell: h3.Cell(c), Area: AreaRef{ID: a.ID, Code: a.Code, Name: a.Name, Country: a.Country}})
		perArea[id] = append(perArea[id], h3.Cell(c))
	}
	for id, cs := range perArea {
		byID[id].CellCount, byID[id].Cells, byID[id].BBox = len(cs), Compact(cs), BBox(cs)
	}
	return areas, cells, crows.Err()
}

// insert creates a DRAFT area.
func (s store) insert(ctx context.Context, tx pgx.Tx, a Area) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO areas (brand_id, country_code, code, name, surge_enabled, surge_max) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		s.brandID, a.Country, a.Code, a.Name, a.SurgeEnabled, a.SurgeMax).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return "", ErrAreaCodeTaken
	}
	return id, err
}

// update writes the editable fields of an area.
func (s store) update(ctx context.Context, tx pgx.Tx, a *Area) error {
	_, err := tx.Exec(ctx, `UPDATE areas SET name=$3, status=$4, surge_enabled=$5, surge_max=$6, updated_at=now() WHERE brand_id=$1 AND id=$2`,
		s.brandID, a.ID, a.Name, a.Status, a.SurgeEnabled, a.SurgeMax)
	return err
}

// replaceCells swaps an area's cell set; a cell owned elsewhere is ErrCellsOverlap (the PK is the final guard).
func (s store) replaceCells(ctx context.Context, tx pgx.Tx, areaID string, cells []h3.Cell) error {
	if _, err := tx.Exec(ctx, `DELETE FROM area_cells WHERE area_id=$1`, areaID); err != nil {
		return err
	}
	if len(cells) == 0 {
		return nil
	}
	rows := make([][]any, len(cells))
	for i, c := range cells {
		rows[i] = []any{s.brandID, int64(c), areaID}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"area_cells"}, []string{"brand_id", "cell", "area_id"}, pgx.CopyFromRows(rows))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrCellsOverlap
	}
	return err
}

// readiness reports what an area still lacks before it can go ACTIVE.
func (s store) readiness(ctx context.Context, tx pgx.Tx, a *Area) ([]string, error) {
	var cells, policies, tariffs int
	err := tx.QueryRow(ctx, `SELECT
		  (SELECT count(*) FROM area_cells WHERE area_id=$2),
		  (SELECT count(*) FROM pricing_profile_assignments WHERE brand_id=$1 AND country_code=$3 AND valid_to IS NULL
		     AND (scope_kind='COUNTRY' OR (scope_kind='AREA' AND scope_id=$2::text))),
		  (SELECT count(*) FROM tariffs t JOIN vehicle_classes vc ON vc.id=t.vehicle_class_id JOIN services sv ON sv.id=vc.service_id
		     WHERE sv.brand_id=$1 AND t.country_code=$3 AND (t.area_id IS NULL OR t.area_id=$2) AND t.status='ACTIVE')`,
		s.brandID, a.ID, a.Country).Scan(&cells, &policies, &tariffs)
	if err != nil {
		return nil, err
	}
	var missing []string
	if cells == 0 {
		missing = append(missing, "cells")
	}
	if policies == 0 {
		missing = append(missing, "fee_policy")
	}
	if tariffs == 0 {
		missing = append(missing, "tariff")
	}
	return missing, nil
}

// inUse counts registered drivers and open trips, which block archiving.
func (s store) inUse(ctx context.Context, tx pgx.Tx, areaID string) (drivers, trips int, err error) {
	err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM driver_areas WHERE area_id=$1),
		(SELECT count(*) FROM trips WHERE area_id=$1 AND status NOT IN ('COMPLETED','CANCELLED_BY_RIDER','CANCELLED_BY_DRIVER','CANCELLED_BY_STAFF','NO_DRIVER_FOUND','EXPIRED'))`,
		areaID).Scan(&drivers, &trips)
	return drivers, trips, err
}

// countryStatus returns a country's status, or "" when unknown to the brand.
func (s store) countryStatus(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, code string) (string, error) {
	var st string
	err := q.QueryRow(ctx, `SELECT status FROM countries WHERE brand_id=$1 AND code=$2`, s.brandID, strings.ToUpper(code)).Scan(&st)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return st, err
}

// driverAreas lists a driver's areas, primary first.
func (s store) driverAreas(ctx context.Context, driverID string) ([]AreaRef, string, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id, a.code, a.name, a.country_code, d.is_primary FROM driver_areas d JOIN areas a ON a.id=d.area_id
		WHERE d.driver_id=$1 ORDER BY d.is_primary DESC, a.name`, driverID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []AreaRef{}
	primary := ""
	for rows.Next() {
		var r AreaRef
		var p bool
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Country, &p); err != nil {
			return nil, "", err
		}
		if p {
			primary = r.ID
		}
		out = append(out, r)
	}
	return out, primary, rows.Err()
}

// setDriverAreas replaces a driver's areas and sets their country.
func (s store) setDriverAreas(ctx context.Context, tx pgx.Tx, driverID, country string, areaIDs []string, primary string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM driver_areas WHERE driver_id=$1`, driverID); err != nil {
		return err
	}
	for _, id := range areaIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO driver_areas (driver_id, area_id, is_primary) VALUES ($1,$2,$3)`, driverID, id, id == primary); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE driver_profiles SET country_code=$2 WHERE user_id=$1`, driverID, country)
	return err
}

// licenceClasses is never NULL in the table.
func licenceClasses(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
