// Package actions is the Platform's catalogue of approvable actions: for each
// one its key, labels, permission, condition field, the payload a gated
// endpoint stores, a describer (the readable diff approvers see, and an early
// refusal when the change cannot apply) and an executor that calls the owning
// module as the requester of record.
//
//   - keys.go        action keys and payload types (what gated endpoints store)
//   - register.go    Register: adds every action whose module is wired
//   - pricing.go     pricing profiles: publish a version, status, default, assign
//   - money.go       trip refunds and driver payouts (amount conditions)
//   - trust.go       privacy deletions and regulator switches
//   - staff.go       staff role changes and branding publication
package actions
