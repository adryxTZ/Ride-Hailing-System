// Package regulator is the port to transport regulators (LATRA in Tanzania):
// registering drivers and vehicles, submitting completed trips and the daily
// report. Adapters live in sub-packages (fake for development and CI, cets for
// LATRA's e-ticketing server); the compliance module owns queueing and retries.
//
// Records are in Platform terms (GeoJSON-order points, decimal money, UTC
// times); every regulator-specific format is applied inside its adapter.
//
// Files: regulator.go (the port, records, acknowledgements and errors).
package regulator
