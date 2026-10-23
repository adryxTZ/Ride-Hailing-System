// Package catalog owns the service catalogue: the taxi service, its vehicle
// classes (Bajaji, Standard, XL …) and request schema. It is read on every app start and quote,
// so it is served from a versioned in-process cache (kernel/cache) and only
// hits Postgres when an admin changes something.
//
//   - model.go    domain types
//   - store.go    persistence: load the catalogue, upsert from seed files, bump the version
//   - service.go  cached read API used by HTTP handlers and (later) quotes/dispatch
package catalog
