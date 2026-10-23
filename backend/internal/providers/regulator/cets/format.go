package cets

import (
	"errors"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // local times must not depend on the host's zoneinfo (slim images)

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// CeTS formats: local time with a space and whole seconds; dates as days.
const (
	dateTimeLayout = "2006-01-02 15:04:05"
	dateLayout     = "2006-01-02"
)

// errNoTimezone is a record whose IANA timezone is missing or unknown.
var errNoTimezone = errors.New("record timezone is missing or unknown")

// location loads the record's IANA timezone; empty is an error, never UTC.
func location(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errNoTimezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, errNoTimezone
	}
	return loc, nil
}

// localDateTime renders t as "YYYY-MM-DD HH:MM:SS" in loc; zero is null.
func localDateTime(t time.Time, loc *time.Location) *string {
	if t.IsZero() {
		return nil
	}
	s := t.In(loc).Format(dateTimeLayout)
	return &s
}

// localDate renders t as "YYYY-MM-DD" in loc; nil or zero is null.
func localDate(t *time.Time, loc *time.Location) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.In(loc).Format(dateLayout)
	return &s
}

// coords renders a point as "lat,lng". (0,0) is a data gap, sent as null
// rather than a point off the African coast.
// CeTS's caption says "Longitude and latitude"; lat-first is the previous
// system's reading and must be confirmed with LATRA in writing. Flip here only.
func coords(p regulator.Point) *string {
	if p.Lat == 0 && p.Lng == 0 {
		return nil
	}
	s := strconv.FormatFloat(p.Lat, 'f', -1, 64) + "," + strconv.FormatFloat(p.Lng, 'f', -1, 64)
	return &s
}

// plate normalises a registration number: upper case, no whitespace.
func plate(v string) *string {
	return text(strings.ToUpper(strings.Join(strings.Fields(v), "")))
}

// wholeAmount rounds money to whole currency units (TZS has no minor unit).
func wholeAmount(d decimal.Decimal) int64 {
	return d.Round(0).IntPart()
}

// positiveAmount is wholeAmount, or null when d is not positive (a missing fare).
func positiveAmount(d decimal.Decimal) *int64 {
	if !d.IsPositive() {
		return nil
	}
	v := wholeAmount(d)
	return &v
}

// text trims s and turns empty into null: gaps are reported, never invented.
func text(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
