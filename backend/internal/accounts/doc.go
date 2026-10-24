// Package accounts is the Finance › Accounts view of the ledger: one account
// per party paid from fares (platform revenue, VAT, levy, each charge payee,
// drivers, riders, the gateway, the platform bank) with its balance, the
// period's movements and, for fare-share parties, the fare base and the
// effective rate. Staff record payments against what the platform owes (VAT,
// levy, a payee, rider refunds); each payment is one balanced, idempotent
// ledger posting from PLATFORM_BANK. Amounts are always in one currency.
//
//   - model.go      accounts, entries, payments, limits and error codes
//   - keys.go       account keys ("VAT_PAYABLE", "CHARGES_PAYABLE:<payee>") and each kind's position
//   - ports.go      the ledger operations a payment needs
//   - service.go    list, entries, export and owed refunds (read use cases)
//   - settle.go     record a payment (validation, overpay check, posting)
//   - allocate.go   which rider refunds a payment covers (pure)
//   - csv.go        CSV export with spreadsheet-formula escaping
//   - store.go      account list and entry queries (one query each)
//   - store_settle.go  payments, owed balances and rider refunds persistence
package accounts
