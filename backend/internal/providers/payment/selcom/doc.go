// Package selcom is the Selcom adapter for Tanzania.
//
//   - selcom.go  Checkout API: collections (USSD push), order status, status mapping
//   - sign.go    Checkout request signing (HMAC-SHA256)
//   - payout.go  Business API: payouts to mobile wallets (RSA-SHA256 signed),
//     transfer query, wallet chosen from the number's prefix
package selcom
