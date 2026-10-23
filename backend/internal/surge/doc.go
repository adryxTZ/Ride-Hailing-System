// Package surge prices busy areas: every RecomputeInterval it compares open
// requests with free drivers per H3 area (resolution 7, ~5 km²) and stores a
// multiplier per area in Redis; quotes read the multiplier at the pickup and
// lock it. Each tariff caps what it accepts ("surge_max"), which keeps fares
// inside the regulator's bands.
//
//   - surge.go      Service: Recompute (dispatcher leader), At (quotes)
//   - multiplier.go the demand/supply → multiplier rule (pure)
package surge
