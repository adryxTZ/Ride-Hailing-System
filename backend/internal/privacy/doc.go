// Package privacy is data protection (spec 2026-10-23-compliance-trust.md, S4):
//
//   - consent: riders and drivers accept a versioned privacy notice per app
//     (texts are brand data); booking, going online and submitting an
//     application need the current version; a new version asks again;
//
//   - requests: a person asks for an export or for deletion; staff approve or
//     reject within the due date; an export is one JSON file behind a 24 h
//     link; deletion anonymises the account (trip, receipt and ledger records
//     are kept as the law requires) and is blocked while money or a trip is open;
//
//   - retention: a daily job deletes or strips data past its period.
//
//   - model.go      types, constants, error codes
//
//   - consent.go    current versions, status, accept, the gate check (Redis, then Postgres)
//
//   - requests.go   create, list, cancel, download; staff approve and reject
//
//   - export.go     building the export file
//
//   - erase.go      deletion blockers and anonymisation
//
//   - retention.go  the retention rules and the daily run
//
//   - store.go      persistence
//
//   - ports.go      money (blockers), files, notifications, presence
//
//   - jobs.go       River jobs: export, retention
package privacy
