// Package identity owns users, roles and sessions: phone OTP sign-in,
// access/refresh token issuance with rotation and reuse detection, and logout.
//
//   - model.go    domain types (User, OTP, RefreshToken) and error codes
//   - ports.go    what this module needs from the outside (SMS, clock, keys)
//   - store.go    persistence (pgx); no business rules
//   - service.go  use cases: RequestOTP, VerifyOTP, Refresh, Logout
//   - jobs.go     River job that sends the OTP text message
//   - access.go   staff limits and sign-in sessions; ending sessions
//   - person.go   a rider's or driver's account for staff: sessions, devices, notes
package identity
