package receipts

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// numberFormat is the receipt number: year and the year's sequence, e.g. 2026-000123.
const numberFormat = "%04d-%06d"

// Number formats a receipt number.
func Number(year, seq int) string { return fmt.Sprintf(numberFormat, year, seq) }

// Build turns a trip's facts into a receipt (without id, number or issue time).
// The lines must add up to what was paid: a receipt that disagrees with the
// money is never issued.
func Build(f Facts, issuer Issuer) (Receipt, error) {
	lines := make([]Line, 0, len(f.Lines))
	sum := decimal.Zero
	for _, l := range f.Lines {
		if l.Amount.IsZero() && l.Code != "BASE_FARE" {
			continue // nothing to show (e.g. no booking fee)
		}
		label := labelFor(l.Code)
		if l.Label["en"] != "" && l.Label["sw"] != "" {
			label = l.Label
		}
		lines = append(lines, Line{Code: l.Code, Label: label, Amount: l.Amount, Meta: l.Meta})
		sum = sum.Add(l.Amount)
	}
	if !sum.Equal(f.Total) {
		return Receipt{}, ErrLinesMismatch.WithDetails(map[string]any{"lines": sum, "total": f.Total, "trip": f.TripID})
	}
	return Receipt{
		TripID: f.TripID, Currency: f.Currency, Total: f.Total, Lines: lines,
		Taxes:   Taxes{VAT: f.VAT, Levy: f.Levy, Included: true},
		Payment: f.Payment, Issuer: issuer, Trip: f.Trip, Driver: f.Driver,
		riderID: f.RiderID, driverID: f.DriverID,
	}, nil
}

// yearIn is the calendar year of t in a timezone (receipts are numbered per local year).
func yearIn(t time.Time, tz string) int {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Year()
}
