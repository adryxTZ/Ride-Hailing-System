// Package payments moves money through a payment gateway: drivers paying their
// cash-trip debt, and riders paying trip fares, by mobile money (USSD push).
//
// A pay-in is PENDING until the gateway's own status says COLLECTED or FAILED;
// webhooks only prompt a status check and are never trusted. On COLLECTED the
// ledger posting and the status change commit in one database transaction.
//
// Files:
//   - model.go   Payin, statuses, failure codes, constants, error codes
//   - ports.go   what this module needs: gateway, ledger, holds, limits, jobs, notifier
//   - store.go   gateway_transactions persistence
//   - service.go use cases: Create, Refresh, Get, List, WebhookRef
//   - fares.go   trip fares: PromptFare (automatic at ride end, or the rider's retry),
//     collectFare (completes the trip and posts the split in one transaction),
//     late money owed back to the rider, FareStateOf for the apps
//   - jobs.go    per-pay-in status check and the periodic reconciliation
package payments
