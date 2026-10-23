package geofences

import (
	"encoding/json"
	"testing"
)

// square is a closed ring around (lng, lat) with half-size d degrees.
func square(lng, lat, d float64) Ring {
	return Ring{{lng - d, lat - d}, {lng + d, lat - d}, {lng + d, lat + d}, {lng - d, lat + d}, {lng - d, lat - d}}
}

func zone(id string, rings ...Ring) Geofence {
	return Geofence{ID: id, Kind: KindZone, Status: StatusActive, Geometry: &Geometry{Type: "MultiPolygon", Coordinates: []Polygon{rings}}}
}

func TestContainsPolygonAndHole(t *testing.T) {
	outer, hole := square(39.25, -6.80, 0.05), square(39.25, -6.80, 0.01)
	ix, err := NewIndex([]Geofence{zone("z", outer, hole)})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Contains(Point{Lat: -6.83, Lng: 39.27})) != 1 {
		t.Fatal("a point inside the ring must hit")
	}
	if len(ix.Contains(Point{Lat: -6.80, Lng: 39.25})) != 0 {
		t.Fatal("a point in the hole must not hit")
	}
	if len(ix.Contains(Point{Lat: -6.90, Lng: 39.25})) != 0 {
		t.Fatal("a point outside must not hit")
	}
	// Near the boundary, inside: the cover must include boundary cells.
	if len(ix.Contains(Point{Lat: -6.7505, Lng: 39.25})) != 1 {
		t.Fatal("a point just inside the edge must hit")
	}
}

func TestPlaceRadius(t *testing.T) {
	r := 800
	ix, _ := NewIndex([]Geofence{{ID: "p", Kind: KindPlace, Status: StatusActive, Center: &Point{Lat: -6.8781, Lng: 39.2026}, RadiusM: &r}})
	const metresPerDegLat = 111_195
	inside := Point{Lat: -6.8781 + 799.0/metresPerDegLat, Lng: 39.2026}
	outside := Point{Lat: -6.8781 + 801.0/metresPerDegLat, Lng: 39.2026}
	if len(ix.Contains(inside)) != 1 || len(ix.Contains(outside)) != 0 {
		t.Fatal("place radius must be exact to the metre")
	}
}

func TestInactiveIsIgnored(t *testing.T) {
	z := zone("z", square(39.25, -6.80, 0.05))
	z.Status = StatusInactive
	ix, _ := NewIndex([]Geofence{z})
	if len(ix.Contains(Point{Lat: -6.80, Lng: 39.25})) != 0 {
		t.Fatal("an inactive geofence must not hit")
	}
}

func TestCrossedNarrowStrip(t *testing.T) {
	// A strip ~60 m wide running north-south at lng 39.25.
	const halfWidth = 0.00027
	strip := Ring{{39.25 - halfWidth, -6.90}, {39.25 + halfWidth, -6.90}, {39.25 + halfWidth, -6.70}, {39.25 - halfWidth, -6.70}, {39.25 - halfWidth, -6.90}}
	ix, _ := NewIndex([]Geofence{zone("strip", strip)})
	across := []Point{{Lat: -6.80, Lng: 39.20}, {Lat: -6.80, Lng: 39.30}}
	before := []Point{{Lat: -6.80, Lng: 39.20}, {Lat: -6.80, Lng: 39.249}}
	if len(ix.Crossed(across)) != 1 {
		t.Fatal("a route across the strip must be detected")
	}
	if len(ix.Crossed(before)) != 0 {
		t.Fatal("a route that stops before the strip must not be detected")
	}
}

func TestParseGeometry(t *testing.T) {
	ok := json.RawMessage(`{"type":"Polygon","coordinates":[[[39.2,-6.8],[39.3,-6.8],[39.3,-6.7],[39.2,-6.8]]]}`)
	if g, err := ParseGeometry(ok); err != nil || g.Type != "MultiPolygon" || g.Vertices() != 4 {
		t.Fatalf("valid polygon refused: %v", err)
	}
	for name, raw := range map[string]string{
		"open ring":        `{"type":"Polygon","coordinates":[[[39.2,-6.8],[39.3,-6.8],[39.3,-6.7],[39.2,-6.7]]]}`,
		"too few points":   `{"type":"Polygon","coordinates":[[[39.2,-6.8],[39.3,-6.8],[39.2,-6.8]]]}`,
		"lat out of range": `{"type":"Polygon","coordinates":[[[39.2,-96.8],[39.3,-6.8],[39.3,-6.7],[39.2,-96.8]]]}`,
		"point":            `{"type":"Point","coordinates":[39.2,-6.8]}`,
	} {
		if _, err := ParseGeometry(json.RawMessage(raw)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
