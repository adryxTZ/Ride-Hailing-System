// Package geofences owns the shapes pricing profiles can be applied to beyond
// regions (spec 2026-10-23-pricing-profiles-geofences.md, S2): districts and
// zones drawn as polygons, and named places as a point with a radius (an
// airport, a port, a mall). Every shape change is a new version.
//
// Saving is validated by PostGIS (staff only, so the database is fine there);
// quotes use an in-memory index — an H3 res-7 cover to find candidates, then
// an exact point-in-polygon or distance test — so finding the geofences at a
// point or along a route costs no database query.
//
//   - model.go     Geofence, Version, Hit, constants, error codes
//   - geometry.go  GeoJSON parsing and ring checks (pure)
//   - index.go     the in-memory index: Contains (a point), Crossed (a route) (pure)
//   - store.go     PostGIS validation and persistence
//   - service.go   list, get, create, reshape, rename/status, overlaps, the cached index
//   - importer.go  GeoJSON FeatureCollection import (dry run, then all-or-nothing)
package geofences
