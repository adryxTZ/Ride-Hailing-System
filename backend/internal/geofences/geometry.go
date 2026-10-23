package geofences

import (
	"encoding/json"
	"fmt"
	"strings"
)

// minRingPoints is a triangle closed back on its first point.
const minRingPoints = 4

// ParseGeometry reads a GeoJSON Polygon or MultiPolygon ([lng, lat] order) and
// checks what can be checked without the database: types, closed rings,
// enough points and coordinate ranges. PostGIS checks self-intersection and size.
func ParseGeometry(raw json.RawMessage) (*Geometry, error) {
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, invalid("WRONG_TYPE", "not GeoJSON")
	}
	var polys []Polygon
	switch g.Type {
	case "Polygon":
		var p Polygon
		if err := json.Unmarshal(g.Coordinates, &p); err != nil {
			return nil, invalid("WRONG_TYPE", "bad Polygon coordinates")
		}
		polys = []Polygon{p}
	case "MultiPolygon":
		if err := json.Unmarshal(g.Coordinates, &polys); err != nil {
			return nil, invalid("WRONG_TYPE", "bad MultiPolygon coordinates")
		}
	default:
		return nil, invalid("WRONG_TYPE", "expected Polygon or MultiPolygon")
	}
	if len(polys) == 0 {
		return nil, invalid("TOO_FEW_POINTS", "no polygon")
	}
	for _, p := range polys {
		if err := checkPolygon(p); err != nil {
			return nil, err
		}
	}
	return &Geometry{Type: "MultiPolygon", Coordinates: polys}, nil
}

func checkPolygon(p Polygon) error {
	if len(p) == 0 {
		return invalid("TOO_FEW_POINTS", "empty polygon")
	}
	for _, r := range p {
		if len(r) < minRingPoints {
			return invalid("TOO_FEW_POINTS", fmt.Sprintf("a ring has %d points", len(r)))
		}
		if r[0] != r[len(r)-1] {
			return invalid("RING_NOT_CLOSED", "the last point must repeat the first")
		}
		for _, c := range r {
			if c[0] < -180 || c[0] > 180 || c[1] < -90 || c[1] > 90 {
				return invalid("WRONG_TYPE", "coordinates must be [lng, lat] in range")
			}
		}
	}
	return nil
}

// Vertices counts every point of every ring.
func (g *Geometry) Vertices() int {
	n := 0
	for _, p := range g.Coordinates {
		for _, r := range p {
			n += len(r)
		}
	}
	return n
}

// WKT renders the geometry for PostGIS (SRID 4326, lng lat order).
func (g *Geometry) WKT() string {
	var b strings.Builder
	b.WriteString("SRID=4326;MULTIPOLYGON(")
	for i, p := range g.Coordinates {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("(")
		for j, r := range p {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("(")
			for k, c := range r {
				if k > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, "%.7f %.7f", c[0], c[1])
			}
			b.WriteString(")")
		}
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

// BBox is west, south, east, north.
func (g *Geometry) BBox() [4]float64 {
	b := [4]float64{180, 90, -180, -90}
	for _, p := range g.Coordinates {
		for _, c := range p[0] {
			b[0], b[1] = min(b[0], c[0]), min(b[1], c[1])
			b[2], b[3] = max(b[2], c[0]), max(b[3], c[1])
		}
	}
	return b
}
