// Package phone normalises user-entered phone numbers to E.164 using the
// brand's country as the default calling code. No external data, just the
// countries the Platform operates in; extend the table as brands are added.
package phone

import (
	"fmt"
	"strings"
)

// callingCodes maps ISO-3166 alpha-2 → calling code with local prefix rules.
var callingCodes = map[string]string{
	"TZ": "255", "KE": "254", "UG": "256", "RW": "250", "ZM": "260", "MW": "265", "MZ": "258",
}

// Normalize returns +<cc><national> or an error. Accepts "+255712345678",
// "255712345678", "0712345678" and "712345678" for country TZ.
func Normalize(raw, country string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	if digits == "" {
		return "", fmt.Errorf("empty phone")
	}
	cc, ok := callingCodes[strings.ToUpper(country)]
	if !ok {
		return "", fmt.Errorf("unsupported country %q", country)
	}
	switch {
	case strings.HasPrefix(raw, "+"):
		// international as typed
	case strings.HasPrefix(digits, cc) && len(digits) >= len(cc)+9:
		// already has the country code without '+'
	case strings.HasPrefix(digits, "0"):
		digits = cc + digits[1:]
	default:
		digits = cc + digits
	}
	if len(digits) < 11 || len(digits) > 15 {
		return "", fmt.Errorf("invalid length")
	}
	return "+" + digits, nil
}
