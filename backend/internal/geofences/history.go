package geofences

import (
	"context"
	"encoding/json"
	"time"
)

// Index returns the cached index of active geofences (in memory).
func (s *Service) Index(ctx context.Context) (*Index, error) {
	ix, _, err := s.index.Get(ctx)
	return ix, err
}

// IndexAt builds an index of a country's shapes as they were at `at`: each
// geofence's newest version saved by then. Archiving is not versioned, so a
// shape archived since still counts; whether a profile applied there at `at`
// is decided by the assignment history, not by the shape.
func (s *Service) IndexAt(ctx context.Context, country string, at time.Time) (*Index, error) {
	fences, err := s.store.shapesAt(ctx, country, at)
	if err != nil {
		return nil, err
	}
	return NewIndex(fences)
}

// shapesAt reads each geofence's version in force at `at` (one query).
func (s store) shapesAt(ctx context.Context, country string, at time.Time) ([]Geofence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (g.id) g.id, g.country_code, g.kind, g.code, g.name, v.version,
		       CASE WHEN v.geom IS NULL THEN NULL ELSE ST_AsGeoJSON(v.geom::geometry, 7) END,
		       CASE WHEN v.center IS NULL THEN NULL ELSE ST_Y(v.center::geometry) END,
		       CASE WHEN v.center IS NULL THEN NULL ELSE ST_X(v.center::geometry) END, v.radius_m
		FROM geofences g JOIN geofence_versions v ON v.geofence_id = g.id
		WHERE g.brand_id = $1 AND g.country_code = $2 AND v.created_at <= $3
		ORDER BY g.id, v.version DESC`, s.brandID, country, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Geofence
	for rows.Next() {
		f := Geofence{Status: StatusActive}
		var geo *string
		var lat, lng *float64
		if err := rows.Scan(&f.ID, &f.Country, &f.Kind, &f.Code, &f.Name, &f.Version, &geo, &lat, &lng, &f.RadiusM); err != nil {
			return nil, err
		}
		if geo != nil {
			if f.Geometry, err = ParseGeometry(json.RawMessage(*geo)); err != nil {
				return nil, err
			}
		}
		if lat != nil && lng != nil {
			f.Center = &Point{Lat: *lat, Lng: *lng}
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
