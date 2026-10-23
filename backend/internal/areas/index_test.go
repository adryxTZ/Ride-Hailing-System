package areas

import (
	"testing"

	"github.com/uber/h3-go/v4"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// A small square around central Dar es Salaam, GeoJSON order.
var darRing = [][2]float64{{39.15, -6.9}, {39.35, -6.9}, {39.35, -6.7}, {39.15, -6.7}, {39.15, -6.9}}

func testIndex(t *testing.T) *Index {
	t.Helper()
	cells, err := PolygonToCells(darRing)
	if err != nil || len(cells) == 0 {
		t.Fatalf("polygon to cells: %d %v", len(cells), err)
	}
	ref := AreaRef{ID: "dar", Code: "dar-es-salaam", Name: "Dar es Salaam", Country: "TZ"}
	rows := make([]cellRow, len(cells))
	for i, c := range cells {
		rows[i] = cellRow{Cell: c, Area: ref}
	}
	return Build([]*Area{{ID: "dar", Country: "TZ", Status: StatusActive}}, rows, map[string]bool{"TZ": true})
}

func TestAreaOf(t *testing.T) {
	ix := testIndex(t)
	if ref, ok := ix.Serviceable(routing.Point{Lat: -6.8, Lng: 39.25}); !ok || ref.Code != "dar-es-salaam" {
		t.Fatalf("point in Dar should resolve, got %+v %v", ref, ok)
	}
	if _, ok := ix.AreaOf(routing.Point{Lat: -6.0, Lng: 40.5}); ok {
		t.Fatal("a point in the ocean must not resolve")
	}
}

func TestDraftAreaIsNotServiceable(t *testing.T) {
	ix := testIndex(t)
	ix.areas["dar"].Status = StatusDraft
	if _, ok := ix.Serviceable(routing.Point{Lat: -6.8, Lng: 39.25}); ok {
		t.Fatal("a DRAFT area must not serve quotes")
	}
	ix.areas["dar"].Status = StatusActive
	ix.active["TZ"] = false
	if _, ok := ix.Serviceable(routing.Point{Lat: -6.8, Lng: 39.25}); ok {
		t.Fatal("an area in a DRAFT country must not serve quotes")
	}
}

func TestExpand(t *testing.T) {
	c, _ := h3.LatLngToCell(h3.LatLng{Lat: -6.8, Lng: 39.25}, 5)
	fine, err := Expand([]string{c.String()})
	if err != nil || len(fine) != 49 {
		t.Fatalf("one res-5 cell expands to 49 res-7 cells, got %d %v", len(fine), err)
	}
	fine7, _ := h3.LatLngToCell(h3.LatLng{Lat: -6.8, Lng: 39.25}, 9)
	if _, err := Expand([]string{fine7.String()}); err == nil {
		t.Fatal("cells finer than res 7 must be refused")
	}
	if _, err := Expand([]string{"not-a-cell"}); err == nil {
		t.Fatal("garbage must be refused")
	}
	if got := Compact(fine); len(got) != 1 || got[0] != c.String() {
		t.Fatalf("compact should fold back to the res-5 cell, got %v", got)
	}
}

func TestOverlaps(t *testing.T) {
	ix := testIndex(t)
	inside, _ := CellOf(routing.Point{Lat: -6.8, Lng: 39.25})
	outside, _ := CellOf(routing.Point{Lat: -5.0, Lng: 39.25})
	conflicts, owners := ix.Overlaps("other", []h3.Cell{inside, outside})
	if len(conflicts) != 1 || owners["dar"].Code != "dar-es-salaam" {
		t.Fatalf("want one conflict owned by Dar, got %v %v", conflicts, owners)
	}
	if c, _ := ix.Overlaps("dar", []h3.Cell{inside}); len(c) != 0 {
		t.Fatal("an area's own cells are not conflicts")
	}
}
