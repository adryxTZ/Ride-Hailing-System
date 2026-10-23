// Package payouts pays drivers what the platform owes them (earnings from
// mobile-money trips, their share of cancellation fees) to their account phone.
//
// A driver asks to cash out; Finance approves each request unless the fee
// policy says payouts are automatic. The money is sent once (marked sent before
// the gateway is called), the gateway's own status is the only truth, and the
// ledger moves only when the gateway says PAID. An unclear gateway reply never
// fails a payout: it stays PENDING until a status check answers, and one still
// pending after StuckAfter is flagged for Finance.
//
// Statuses: REQUESTED (waiting for Finance) → PENDING (with the gateway) →
// PAID | FAILED; REQUESTED → REJECTED (Finance) | CANCELLED (driver).
//
// Files:
//   - model.go   Payout, statuses, limits, error codes
//   - ports.go   what this module needs: ledger, payout gateway, rules, holds, people, jobs
//   - store.go   driver_payouts persistence
//   - service.go the driver's side: Create, Cancel, Get, List, Wallet
//   - admin.go   Finance's side: List, Approve, Reject, Totals
//   - send.go    sending (send), status checks (Refresh) and the reconciliation sweep
//   - jobs.go    River workers: send, check, reconcile
package payouts
