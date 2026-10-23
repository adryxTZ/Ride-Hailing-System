// Package cets is the regulator adapter for LATRA's Core e-Ticketing System
// (CeTS, API doc v2.1.1). Every endpoint is a POST of a JSON body with a
// self-signed RS256 bearer token, answered by one envelope:
//
//	{"status": "MS", "trxnId": null, "statusCode": 5000, "message": "Successful", "data": {...}}
//
// Success is statusCode 5000 only; a 2xx with any other code is a rejection
// classified by codes.go. Transport failures, non-2xx statuses and unreadable
// bodies are regulator.ErrUnavailable. The adapter never retries: the
// compliance sweeper owns the retry ladder.
//
// All CeTS formatting happens here, at the edge: local "YYYY-MM-DD HH:MM:SS"
// times in the record's timezone, "lat,lng" coordinates, plates upper-cased
// without spaces and integer money.
//
// Files:
//   - client.go    Config, New, the four port methods, HTTP and envelope handling
//   - codes.go     CeTS status-code table: success, rejection classes
//   - jwt.go       RS256 token signing (stdlib only), key parsing, JWKS document
//   - payloads.go  the four request bodies, field names verbatim from CeTS
//   - format.go    time, date, coordinate, plate and amount formatting
//
// Tests use an httptest stub; golden request bodies live in testdata/
// (regenerate with go test ./internal/providers/regulator/cets -update).
package cets
