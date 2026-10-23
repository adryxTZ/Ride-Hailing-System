// Package profiles owns pricing profiles (DOC/specs/2026-10-23-pricing-profiles-geofences.md):
// the money rules a trip is priced under (commission, booking fee, rounding,
// waiting cap, cancellation terms, and on a country's default the tax, debt and
// payout rules), the extra charges a profile adds (toll, parking, levy …), where
// each profile applies (the country default, a region; later districts, zones,
// places) and the immutable pricing record every trip keeps.
//
// Quotes resolve a profile from an in-memory snapshot (no database query on the
// hot path); versions are immutable and picked by effective time; assignments
// are only ever closed, never edited, so "which profile applied here, when" is
// always answerable.
//
//   - model.go     types, constants, error codes
//   - terms.go     Terms (nullable = inherit), Merge, validation, content hash
//   - charges.go   ChargeRule, validation, ApplyCharges (pure)
//   - resolve.go   Snapshot and Resolve (pure): which profile wins at a point
//   - snapshot.go  loading the snapshot (cached by the pricing config version)
//   - store.go     admin persistence
//   - service.go   admin use cases: profiles, versions, status, assignments, default
//   - payees.go    third parties a charge is owed to
//   - records.go   per-trip pricing records: pin at booking, finalise at completion
//   - ports.go     what this package needs from the catalogue and areas
package profiles
