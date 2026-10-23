// Package fake is a regulator adapter for local development and CI. It makes no
// network calls; the outcome of every submission is chosen, in this order, by:
//
//  1. a queued Go-test outcome for the submission's kind (Script);
//  2. a trigger in the record's data:
//     plate prefix TFAIL       rejected with code 5001 (RETRY)
//     licence 9999999999       rejected with code 5001 (RETRY)
//     plate prefix TDOWN       regulator.ErrUnavailable
//  3. the adapter's mode: ok (accepted, reference FAKE-XXXXXXXX), reject
//     (5001, RETRY) or down (regulator.ErrUnavailable).
//
// Files:
//   - fake.go      the adapter, its modes and the four port methods
//   - triggers.go  data triggers read from records
//   - script.go    per-kind outcome queues for Go tests (concurrency-safe)
package fake
