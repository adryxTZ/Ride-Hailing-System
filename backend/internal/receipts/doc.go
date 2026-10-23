// Package receipts issues one immutable, numbered receipt per completed trip
// and serves it as JSON (both apps) and as a PDF in English or Swahili through a
// short-lived signed link.
//
// A receipt is issued inside the transaction that completes the trip, so a
// completed trip never exists without one. Numbers are gapless per brand and
// year (<YYYY>-<6 digits>): the counter row is taken in that same transaction,
// so a rollback gives the number back. The database refuses any change to an
// issued receipt.
//
// Files:
//   - model.go   Receipt and its parts, Facts (what the trip provides), error codes
//   - build.go   Build: lines with labels, the sum check, taxes, payment, snapshots
//   - trip.go    FactsFromTrip: a completed trip as a receipt needs it
//   - labels.go  English and Swahili wording for lines and the PDF
//   - store.go   receipts and receipt_counters persistence
//   - service.go IssueTx, ForTrip (participants and staff), Link, PDF
//   - sign.go    signed PDF links (HMAC, short expiry, constant-time check)
//   - render.go  the A4 PDF (kernel/pdf)
package receipts
