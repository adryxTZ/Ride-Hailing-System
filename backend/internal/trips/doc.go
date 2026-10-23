// Package trips owns the booking and the trip state machine.
//
//   - model.go      Trip, Stop, Event, statuses, error codes
//   - machine.go    the one state machine, parameterised by the lifecycle profile
//   - store.go      persistence (trips, stops, events) — no rules
//   - service.go    use cases: Create (from a quote), Get, Active, Events, Cancel
//   - dispatch_ops.go  what dispatch and drivers do to a trip: offer, release, assign,
//     expire, driver transitions (incl. cancel → re-dispatch); OnChanged/Notify
//   - payment.go    fare payment states and rules: the fare is fixed when the ride
//     ends, cash only when no prompt is open, the read model both apps show
//   - payment_ops.go  what the payments module and the rider do to a fare:
//     FareFor, FarePendingTx, FareFailed, ChooseCash, CompletePaidTx
//
// Dispatch (offers, matching) and realtime fan-out consume trip_events; they
// never mutate a trip except through this package.
package trips
