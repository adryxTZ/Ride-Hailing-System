// Package areas defines where the Platform operates: a brand's countries, each
// divided into areas (regions, counties, states) that staff mark on the map as
// sets of H3 res-7 cells. Prices, surge, dispatch and dashboards are scoped by
// the area a pickup falls in.
//
// Files:
//   - model.go   Country, Area, AreaRef, statuses, limits, error codes
//   - index.go   the pure in-process cell index (resolve a point to its area, expand and overlap checks)
//   - ports.go   what this module needs (audit, config versions)
//   - store.go   countries, areas, area_cells and driver_areas persistence
//   - service.go use cases: Resolve, Countries, List, Create, Update, SaveCells, SetDriverAreas
package areas
