package cets

import (
	"strconv"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// CeTS status codes the adapter understands.
const (
	codeSuccess = 5000 // accepted; the only success code
	// codeEmptyReport: a daily report with zero trips. Verified on the CeTS
	// test server by the previous system; the day is closed locally.
	codeEmptyReport = 5016
	// codeUnknownSubjectA and codeUnknownSubjectB are undocumented; read as
	// "driver or vehicle not registered". To be confirmed on the test server.
	codeUnknownSubjectA = 5018
	codeUnknownSubjectB = 5019
)

// maxRawMessage bounds how much of a raw body is kept in a rejection message.
const maxRawMessage = 2048

// classify maps a non-success CeTS code to a rejection class.
func classify(code int, report bool) string {
	switch {
	case code == codeEmptyReport && report:
		return regulator.ClassNotNeeded
	case code == codeUnknownSubjectA, code == codeUnknownSubjectB:
		return regulator.ClassPrerequisite
	}
	return regulator.ClassRetry
}

// rejection builds the port's rejection for a 2xx envelope that is not 5000.
// PREREQUISITE keeps the raw body, since those codes are undocumented.
func rejection(env envelope, raw []byte, report bool) *regulator.Rejection {
	class := classify(env.StatusCode.int(), report)
	msg := env.Message
	if class == regulator.ClassPrerequisite {
		msg = truncate(string(raw), maxRawMessage)
	}
	return &regulator.Rejection{Code: strconv.Itoa(env.StatusCode.int()), Message: msg, Class: class}
}

// truncate cuts s to at most n bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
