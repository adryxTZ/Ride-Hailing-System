// Package auth issues and verifies the Platform's access tokens.
//
//   - keys.go    Ed25519 signing keys: load from disk, or generate ephemeral
//     keys for development; publishes the JWKS document
//   - tokens.go  access-token claims, issue and verify (EdDSA, kid-pinned)
//
// Refresh tokens are opaque and live in the identity module; this package
// knows nothing about users or the database.
package auth
