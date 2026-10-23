// Package pricing turns tariffs and a route into a quote, and (later) a bill.
//
//   - model.go    quote request/response types
//   - store.go    tariff + fee-policy loading (versioned cache), seeding, quote persistence
//   - service.go  the Quote use case: validate attributes, pick eligible classes,
//     route, price every class with engine, persist, return
//
// The arithmetic itself lives in pricing/engine and is pure.
package pricing
