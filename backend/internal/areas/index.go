package areas

import (
	"github.com/uber/h3-go/v4"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Index answers "which area is this point in?" from memory: one map lookup on
// the point's res-7 cell. It is rebuilt whenever areas change (config version).
type Index struct {
	byCell  map[h3.Cell]AreaRef
	areas   map[string]*Area  // by id, every non-archived area
	country map[string]string // area id → country code
	active  map[string]bool   // country code → ACTIVE
}

// cellRow is one stored cell with the area it belongs to.
type cellRow struct {
	Cell h3.Cell
	Area AreaRef
}

// Build creates an index from areas (with their status) and their cells.
func Build(areas []*Area, cells []cellRow, activeCountries map[string]bool) *Index {
	ix := &Index{byCell: make(map[h3.Cell]AreaRef, len(cells)), areas: map[string]*Area{}, country: map[string]string{}, active: activeCountries}
	for _, a := range areas {
		ix.areas[a.ID] = a
		ix.country[a.ID] = a.Country
	}
	for _, c := range cells {
		ix.byCell[c.Cell] = c.Area
	}
	return ix
}

// CellOf is the res-7 cell for a point.
func CellOf(p routing.Point) (h3.Cell, error) {
	return h3.LatLngToCell(h3.LatLng{Lat: p.Lat, Lng: p.Lng}, Resolution)
}

// AreaOf returns the area a point falls in, whatever its status (false when none).
func (ix *Index) AreaOf(p routing.Point) (AreaRef, bool) {
	c, err := CellOf(p)
	if err != nil {
		return AreaRef{}, false
	}
	ref, ok := ix.byCell[c]
	return ref, ok
}

// Serviceable returns the ACTIVE area of an ACTIVE country a point falls in.
func (ix *Index) Serviceable(p routing.Point) (AreaRef, bool) {
	ref, ok := ix.AreaOf(p)
	if !ok {
		return AreaRef{}, false
	}
	a := ix.areas[ref.ID]
	if a == nil || a.Status != StatusActive || !ix.active[a.Country] {
		return AreaRef{}, false
	}
	return ref, true
}

// Area returns a known (non-archived) area by id.
func (ix *Index) Area(id string) (*Area, bool) {
	a, ok := ix.areas[id]
	return a, ok
}

// Overlaps lists the given cells that already belong to another area.
func (ix *Index) Overlaps(areaID string, cells []h3.Cell) (conflicts []h3.Cell, owners map[string]AreaRef) {
	owners = map[string]AreaRef{}
	for _, c := range cells {
		if ref, ok := ix.byCell[c]; ok && ref.ID != areaID {
			conflicts = append(conflicts, c)
			owners[ref.ID] = ref
		}
	}
	return conflicts, owners
}

// Expand validates compacted cell strings (any resolution up to 7) and returns
// their res-7 cells, without duplicates.
func Expand(cells []string) ([]h3.Cell, error) {
	if len(cells) > MaxCellsPerRequest {
		return nil, ErrAreaTooLarge.WithDetails(map[string]any{"max": MaxCellsPerRequest})
	}
	var bad []string
	coarse := make([]h3.Cell, 0, len(cells))
	for _, s := range cells {
		c := h3.Cell(h3.IndexFromString(s))
		if !c.IsValid() {
			bad = append(bad, s)
			continue
		}
		if c.Resolution() > Resolution {
			return nil, ErrCellResolution
		}
		coarse = append(coarse, c)
	}
	if len(bad) > 0 {
		return nil, ErrCellInvalid.WithDetails(map[string]any{"cells": bad})
	}
	fine, err := h3.UncompactCells(coarse, Resolution)
	if err != nil {
		return nil, ErrCellInvalid
	}
	seen := make(map[h3.Cell]struct{}, len(fine))
	out := fine[:0]
	for _, c := range fine {
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	if len(out) > MaxCellsPerArea {
		return nil, ErrAreaTooLarge.WithDetails(map[string]any{"max": MaxCellsPerArea})
	}
	return out, nil
}

// Compact returns the smallest set of (coarser) cells covering the same ground, as strings.
func Compact(cells []h3.Cell) []string {
	packed, err := h3.CompactCells(cells)
	if err != nil {
		packed = cells
	}
	out := make([]string, len(packed))
	for i, c := range packed {
		out[i] = c.String()
	}
	return out
}

// BBox is the bounding box (west, south, east, north) of a cell set.
func BBox(cells []h3.Cell) [4]float64 {
	if len(cells) == 0 {
		return [4]float64{}
	}
	b := [4]float64{180, 90, -180, -90}
	for _, c := range cells {
		boundary, err := c.Boundary()
		if err != nil {
			continue
		}
		for _, v := range boundary {
			b[0], b[1], b[2], b[3] = min(b[0], v.Lng), min(b[1], v.Lat), max(b[2], v.Lng), max(b[3], v.Lat)
		}
	}
	return b
}

// PolygonToCells converts a closed ring of [lng, lat] points (GeoJSON order) to res-7 cells.
func PolygonToCells(ring [][2]float64) ([]h3.Cell, error) {
	loop := make(h3.GeoLoop, 0, len(ring))
	for _, p := range ring {
		loop = append(loop, h3.LatLng{Lat: p[1], Lng: p[0]})
	}
	return h3.PolygonToCells(h3.GeoPolygon{GeoLoop: loop}, Resolution)
}
