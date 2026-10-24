// Package fraud raises flags for staff to review (spec 2026-10-23-compliance-trust.md, S3).
// Three detectors, none of which punishes anyone on its own; staff act through
// the existing suspend, hold and block actions:
//
//   - fake GPS: every driver ping is checked against the previous one in Redis
//     (impossible speed, or the phone's own "mock location" flag); a burst of
//     strikes enqueues one job, so Postgres is never touched on the ping path;
//   - self-booking: when a trip completes, the rider and driver sharing an app
//     install, or the passenger or receiver phone being the driver's;
//   - cancellation abuse: a daily scan of driver cancels, driver no-shows and
//     rider cancels over the last 7 days.
//
// A flag is deduplicated per rule, subject and period (repeats add hits) and is
// frozen once reviewed.
//
//   - model.go       Flag, rules, severities, constants, error codes
//   - gps.go         CheckPing (pure)
//   - gps_guard.go   the Redis strike counter on the ping path
//   - selfbooking.go SelfBookingSignals (pure)
//   - cancel.go      the cancellation thresholds (pure) and the scan
//   - store.go       flags, installs and the queries the detectors read
//   - service.go     raise, list, get, review, trip check
//   - jobs.go        River jobs: gps flag, trip check, hourly cancellation scan
package fraud
