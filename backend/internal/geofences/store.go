package geofences

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// store holds the queries; no rules here.
type store struct {
	pool    *pgxpool.Pool
	brandID string
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Version implements cache.VersionSource for the "geofences" scope.
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

func bump(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='geofences'`)
	return err
}

const fenceColumns = `g.id, g.country_code, g.kind, g.code, g.name, g.status, g.version,
	CASE WHEN g.geom IS NULL THEN NULL ELSE ST_AsGeoJSON(g.geom::geometry, 7) END,
	CASE WHEN g.center IS NULL THEN NULL ELSE ST_Y(g.center::geometry) END, CASE WHEN g.center IS NULL THEN NULL ELSE ST_X(g.center::geometry) END,
	g.radius_m, coalesce(g.area_m2,0), coalesce(g.vertex_count,0), g.updated_at`

func scanFence(row pgx.Row) (Geofence, error) {
	var f Geofence
	var geo *string
	var lat, lng *float64
	err := row.Scan(&f.ID, &f.Country, &f.Kind, &f.Code, &f.Name, &f.Status, &f.Version, &geo, &lat, &lng, &f.RadiusM, &f.AreaM2, &f.VertexCount, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrNotFound
	}
	if err != nil {
		return f, err
	}
	if geo != nil {
		g, err := ParseGeometry(json.RawMessage(*geo))
		if err != nil {
			return f, err
		}
		f.Geometry, f.BBox = g, g.BBox()
	}
	if lat != nil && lng != nil {
		f.Center = &Point{Lat: *lat, Lng: *lng}
		f.BBox = [4]float64{*lng, *lat, *lng, *lat}
	}
	return f, nil
}

func (s store) list(ctx context.Context, q querier, where string, args ...any) ([]Geofence, error) {
	rows, err := q.Query(ctx, `SELECT `+fenceColumns+` FROM geofences g WHERE g.brand_id=$1 AND `+where+` ORDER BY g.kind, g.name`, append([]any{s.brandID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Geofence{}
	for rows.Next() {
		f, err := scanFence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s store) get(ctx context.Context, q querier, id string, lock bool) (Geofence, error) {
	sql := `SELECT ` + fenceColumns + ` FROM geofences g WHERE g.id::text=$1 AND g.brand_id=$2`
	if lock {
		sql += ` FOR UPDATE`
	}
	return scanFence(q.QueryRow(ctx, sql, id, s.brandID))
}

// measured is what PostGIS says about a shape.
type measured struct {
	valid    bool
	reason   string
	areaM2   float64
	vertices int
	centroid Point
}

// measure validates a polygon in PostGIS and returns its true area (m²) and centroid.
func (s store) measure(ctx context.Context, q querier, wkt string) (measured, error) {
	var m measured
	err := q.QueryRow(ctx, `SELECT ST_IsValid(g), ST_IsValidReason(g), ST_Area(g::geography), ST_NPoints(g),
		ST_Y(ST_PointOnSurface(g)), ST_X(ST_PointOnSurface(g)) FROM (SELECT ST_GeomFromEWKT($1) AS g) x`, wkt).
		Scan(&m.valid, &m.reason, &m.areaM2, &m.vertices, &m.centroid.Lat, &m.centroid.Lng)
	return m, err
}

// shapeArgs are the geometry columns for a polygon or a place.
type shapeArgs struct {
	wkt      *string
	center   *string
	radiusM  *int
	areaM2   float64
	vertices int
}

func (s store) insert(ctx context.Context, tx pgx.Tx, staffID string, f Geofence, sh shapeArgs, reason string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO geofences (brand_id, country_code, kind, code, name, geom, center, radius_m, area_m2, vertex_count, created_by)
		VALUES ($1,$2,$3,$4,$5,ST_GeogFromText($6),ST_GeogFromText($7),$8,$9,$10,NULLIF($11,'')::uuid) RETURNING id`,
		s.brandID, f.Country, f.Kind, f.Code, f.Name, sh.wkt, sh.center, sh.radiusM, sh.areaM2, sh.vertices, staffID).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return "", ErrCodeTaken
	}
	if err != nil {
		return "", err
	}
	return id, s.addVersion(ctx, tx, id, 1, staffID, sh, reason)
}

func (s store) reshape(ctx context.Context, tx pgx.Tx, staffID string, f Geofence, sh shapeArgs, reason string) error {
	next := f.Version + 1
	if _, err := tx.Exec(ctx, `UPDATE geofences SET version=$2, geom=ST_GeogFromText($3), center=ST_GeogFromText($4), radius_m=$5, area_m2=$6, vertex_count=$7, updated_at=now()
		WHERE id=$1`, f.ID, next, sh.wkt, sh.center, sh.radiusM, sh.areaM2, sh.vertices); err != nil {
		return err
	}
	return s.addVersion(ctx, tx, f.ID, next, staffID, sh, reason)
}

func (s store) addVersion(ctx context.Context, tx pgx.Tx, id string, version int, staffID string, sh shapeArgs, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO geofence_versions (geofence_id, version, geom, center, radius_m, created_by, reason)
		VALUES ($1,$2,ST_GeogFromText($3),ST_GeogFromText($4),$5,NULLIF($6,'')::uuid,$7)`, id, version, sh.wkt, sh.center, sh.radiusM, staffID, reason)
	return err
}

func (s store) versions(ctx context.Context, id string) ([]Version, error) {
	rows, err := s.pool.Query(ctx, `SELECT version, CASE WHEN geom IS NULL THEN NULL ELSE ST_AsGeoJSON(geom::geometry, 7) END,
		CASE WHEN center IS NULL THEN NULL ELSE ST_Y(center::geometry) END, CASE WHEN center IS NULL THEN NULL ELSE ST_X(center::geometry) END,
		radius_m, created_by::text, reason, created_at FROM geofence_versions WHERE geofence_id::text=$1 ORDER BY version DESC LIMIT $2`, id, versionsLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Version{}
	for rows.Next() {
		var v Version
		var geo *string
		var lat, lng *float64
		if err := rows.Scan(&v.Version, &geo, &lat, &lng, &v.RadiusM, &v.CreatedBy, &v.Reason, &v.CreatedAt); err != nil {
			return nil, err
		}
		if geo != nil {
			v.Geometry, _ = ParseGeometry(json.RawMessage(*geo))
		}
		if lat != nil && lng != nil {
			v.Center = &Point{Lat: *lat, Lng: *lng}
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Overlap is two active geofences whose shapes meet.
type Overlap struct {
	A        Hit    `json:"a"`
	B        Hit    `json:"b"`
	Relation string `json:"kind"` // SAME_KIND | NESTED
}

// overlaps finds active pairs in a country whose shapes meet (one query; GiST indexes).
func (s store) overlaps(ctx context.Context, country, onlyID string) ([]Overlap, error) {
	rows, err := s.pool.Query(ctx, `
		WITH f AS (SELECT id, kind, name, version, country_code, coalesce(geom::geography, ST_Buffer(center, radius_m)) AS shape
		           FROM geofences WHERE brand_id=$1 AND country_code=$2 AND status='ACTIVE')
		SELECT a.id, a.kind, a.name, a.version, b.id, b.kind, b.name, b.version, CASE WHEN a.kind=b.kind THEN 'SAME_KIND' ELSE 'NESTED' END, a.country_code
		FROM f a JOIN f b ON a.id < b.id AND ST_Intersects(a.shape, b.shape)
		WHERE $3 = '' OR a.id::text = $3 OR b.id::text = $3`, s.brandID, country, onlyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Overlap{}
	for rows.Next() {
		var o Overlap
		if err := rows.Scan(&o.A.ID, &o.A.Kind, &o.A.Name, &o.A.Version, &o.B.ID, &o.B.Kind, &o.B.Name, &o.B.Version, &o.Relation, &o.A.Country); err != nil {
			return nil, err
		}
		o.B.Country = o.A.Country
		out = append(out, o)
	}
	return out, rows.Err()
}
