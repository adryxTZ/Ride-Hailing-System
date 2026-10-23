// Package money owns how a fare is divided. Split is the only place a fare is
// split: rider total = driver earnings + platform net + VAT + levy, checked on
// every call. The ledger, wallets and payouts build on these numbers.
//
//	split.go       Policy, Breakdown and Split
//	split_test.go  the invariant, VAT/levy bases and rounding
//	ledger.go      double-entry, hash-chained ledger: Post (idempotent), PostCashTripCompleted,
//	               DriverWallet, Verify
//	fare.go        trips paid by mobile money (PostTripPaidTx) and late fare money (PostFareUnappliedTx)
//	payin.go       driver debt paid by mobile money
//	payout.go      what a driver may cash out (DriverOwed) and paid payouts (PostPayoutPaidTx)
//	cancellation.go  cancellation fees and the rider dues they create
//	admin.go       finance summary, driver balances, journal (staff views)
package money
