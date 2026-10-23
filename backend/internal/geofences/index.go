package geofences

import (
	"math"

	"github.com/uber/h3-go/v4"
)

// earthRadiusM is the mean Earth radius for haversine distances.
const earthRadiusM = 6_371_000

// Index finds geofences at a point or along a route in memory: an H3 res-7
// cover gives candidates, an exact test decides. Built once per geofence change.
type Index struct {
	byCell map[h3.Cell][]int
	fences []indexed
}

type indexed struct {
	hit     Hit
	polys   []Polygon
	bbox    [4]float64
	center  *Point
	radiusM float64
}

// NewIndex builds the index over active geofences.
func NewIndex(fences []Geofence) (*Index, error) {
	ix := &Index{byCell: map[h3.Cell][]int{}}
	for _, f := range fences {
		if f.Status != StatusActive {
			continue
		}
		e := indexed{hit: Hit{ID: f.ID, Kind: f.Kind, Name: f.Name, Version: f.Version, Country: f.Country}}
		cells, err := cover(f, &e)
		if err != nil {
			return nil, err
		}
		pos := len(ix.fences)
		ix.fences = append(ix.fences, e)
		for _, c := range cells {
			ix.byCell[c] = append(ix.byCell[c], pos)
		}
	}
	return ix, nil
}

// cover is a conservative set of res-7 cells touching the shape.
func cover(f Geofence, e *indexed) ([]h3.Cell, error) {
	if f.Kind == KindPlace && f.Center != nil && f.RadiusM != nil {
		e.center, e.radiusM = f.Center, float64(*f.RadiusM)
		origin, err := h3.LatLngToCell(h3.LatLng{Lat: f.Center.Lat, Lng: f.Center.Lng}, IndexResolution)
		if err != nil {
			return nil, err
		}
		return origin.GridDisk(int(math.Ceil(e.radiusM/cellEdgeM)) + 1)
	}
	if f.Geometry == nil {
		return nil, nil
	}
	e.polys, e.bbox = f.Geometry.Coordinates, f.Geometry.BBox()
	seen := map[h3.Cell]bool{}
	var out []h3.Cell
	add := func(cs []h3.Cell) {
		for _, c := range cs {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	for _, p := range e.polys {
		cells, err := h3.PolygonToCellsExperimental(geoPolygon(p), IndexResolution, h3.ContainmentOverlapping)
		if err != nil {
			return nil, err
		}
		add(cells)
	}
	return out, nil
}

func geoPolygon(p Polygon) h3.GeoPolygon {
	loop := func(r Ring) h3.GeoLoop {
		l := make(h3.GeoLoop, 0, len(r))
		for _, c := range r[:len(r)-1] { // H3 loops are open
			l = append(l, h3.LatLng{Lat: c[1], Lng: c[0]})
		}
		return l
	}
	g := h3.GeoPolygon{GeoLoop: loop(p[0])}
	for _, hole := range p[1:] {
		g.Holes = append(g.Holes, loop(hole))
	}
	return g
}

// Contains returns the geofences containing p (any kind, active only).
func (ix *Index) Contains(p Point) []Hit {
	if ix == nil {
		return nil
	}
	cell, err := h3.LatLngToCell(h3.LatLng{Lat: p.Lat, Lng: p.Lng}, IndexResolution)
	if err != nil {
		return nil
	}
	var out []Hit
	for _, i := range ix.byCell[cell] {
		if ix.fences[i].contains(p) {
			out = append(out, ix.fences[i].hit)
		}
	}
	return out
}

func (e indexed) contains(p Point) bool {
	if e.center != nil {
		return haversineM(*e.center, p) <= e.radiusM
	}
	for _, poly := range e.polys {
		if inPolygon(poly, p) {
			return true
		}
	}
	return false
}

// Crossed returns the zones a route passes through (a segment crosses an edge,
// or a route point lies inside). Places and districts are not "crossed".
func (ix *Index) Crossed(route []Point) []Hit {
	if ix == nil || len(route) < 2 || len(route) > MaxRoutePoints {
		return nil
	}
	rb := routeBBox(route)
	var out []Hit
	for _, e := range ix.fences {
		if e.hit.Kind != KindZone || !bboxMeets(rb, e.bbox) {
			continue
		}
		if e.crossedBy(route) {
			out = append(out, e.hit)
		}
	}
	return out
}

func (e indexed) crossedBy(route []Point) bool {
	for i, p := range route {
		if e.contains(p) {
			return true
		}
		if i == 0 {
			continue
		}
		for _, poly := range e.polys {
			for _, ring := range poly {
				for k := 1; k < len(ring); k++ {
					if segmentsCross(route[i-1], p, ring[k-1], ring[k]) {
						return true
					}
				}
			}
		}
	}
	return false
}

// inPolygon is the even-odd rule: inside the outer ring and outside every hole.
func inPolygon(p Polygon, pt Point) bool {
	if !inRing(p[0], pt) {
		return false
	}
	for _, hole := range p[1:] {
		if inRing(hole, pt) {
			return false
		}
	}
	return true
}

func inRing(r Ring, pt Point) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		xi, yi, xj, yj := r[i][0], r[i][1], r[j][0], r[j][1]
		if (yi > pt.Lat) != (yj > pt.Lat) && pt.Lng < (xj-xi)*(pt.Lat-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// segmentsCross reports whether route segment a–b crosses edge c–d (lng/lat plane).
func segmentsCross(a, b Point, c, d [2]float64) bool {
	orient := func(px, py, qx, qy, rx, ry float64) float64 { return (qx-px)*(ry-py) - (qy-py)*(rx-px) }
	o1 := orient(a.Lng, a.Lat, b.Lng, b.Lat, c[0], c[1])
	o2 := orient(a.Lng, a.Lat, b.Lng, b.Lat, d[0], d[1])
	o3 := orient(c[0], c[1], d[0], d[1], a.Lng, a.Lat)
	o4 := orient(c[0], c[1], d[0], d[1], b.Lng, b.Lat)
	return (o1 > 0) != (o2 > 0) && (o3 > 0) != (o4 > 0)
}

func haversineM(a, b Point) float64 {
	rad := math.Pi / 180
	dLat, dLng := (b.Lat-a.Lat)*rad, (b.Lng-a.Lng)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}

func routeBBox(route []Point) [4]float64 {
	b := [4]float64{180, 90, -180, -90}
	for _, p := range route {
		b[0], b[1], b[2], b[3] = min(b[0], p.Lng), min(b[1], p.Lat), max(b[2], p.Lng), max(b[3], p.Lat)
	}
	return b
}

func bboxMeets(a, b [4]float64) bool {
	return a[0] <= b[2] && b[0] <= a[2] && a[1] <= b[3] && b[1] <= a[3]
}
