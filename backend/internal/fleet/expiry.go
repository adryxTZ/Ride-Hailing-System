package fleet

import (
	"strings"
	"time"
)

// Expiry states of a document, as both the driver and staff see them.
const (
	ExpiryValid      = "VALID"
	ExpiryExpiring   = "EXPIRING" // within ExpiringWithin days
	ExpiryExpired    = "EXPIRED"  // the day after the expiry date
	ExpiryUnknown    = "UNKNOWN"  // tracked but no date given
	ExpiryNotTracked = "NOT_TRACKED"
)

// Expiry rules.
const (
	ExpiringWithin = 30 // days
	MaxExpiryYears = 10
	dateOnly       = "2006-01-02"
)

// ExpiryState says where a document stands on a local day; a document is
// valid through its expiry date. daysLeft is nil when no date applies.
func ExpiryState(tracked bool, expiresOn *string, today time.Time) (string, *int) {
	if !tracked {
		return ExpiryNotTracked, nil
	}
	if expiresOn == nil || *expiresOn == "" {
		return ExpiryUnknown, nil
	}
	exp, err := time.Parse(dateOnly, (*expiresOn)[:min(len(*expiresOn), len(dateOnly))])
	if err != nil {
		return ExpiryUnknown, nil
	}
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	left := int(exp.Sub(day).Hours() / 24)
	switch {
	case left < 0:
		return ExpiryExpired, &left
	case left <= ExpiringWithin:
		return ExpiryExpiring, &left
	}
	return ExpiryValid, &left
}

// checkExpiry validates an upload's expiry date against the type: required when
// the type says so, today or later, at most MaxExpiryYears ahead. Kinds that do
// not track expiry ignore the date.
func checkExpiry(t *DocumentType, expiresOn *string, today time.Time) (*string, error) {
	if !t.TracksExpiry {
		return nil, nil
	}
	if expiresOn == nil || strings.TrimSpace(*expiresOn) == "" {
		if t.ExpiryRequired {
			return nil, fieldError("expires_on", "REQUIRED")
		}
		return nil, nil
	}
	exp, err := time.Parse(dateOnly, strings.TrimSpace(*expiresOn))
	if err != nil {
		return nil, fieldError("expires_on", "FORMAT")
	}
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	switch {
	case exp.Before(day):
		return nil, fieldError("expires_on", "PAST")
	case exp.After(day.AddDate(MaxExpiryYears, 0, 0)):
		return nil, fieldError("expires_on", "TOO_FAR")
	}
	v := exp.Format(dateOnly)
	return &v, nil
}
