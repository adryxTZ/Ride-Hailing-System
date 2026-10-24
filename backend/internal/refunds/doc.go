// Package refunds is staff refunds on completed trips (dashboard): an amount up
// to what the rider paid (less earlier refunds), with a reason, borne by the
// platform or recovered from the driver. The refund and its ledger posting are
// one transaction; the money is owed to the rider (RIDER_REFUNDS) until Finance
// pays it out.
//
//   - model.go    Refund, bearers, error codes
//   - ports.go    the trip facts and the ledger posting this module needs
//   - service.go  create and list
//   - store.go    persistence
package refunds
