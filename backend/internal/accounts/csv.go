package accounts

import (
	"encoding/csv"
	"io"
	"strings"
	"time"
)

// csvHeader is the export's first row.
var csvHeader = []string{"date", "trip", "kind", "description", "holder", "reference", "amount", "running_balance", "currency"}

// formulaLead are first characters a spreadsheet reads as a formula (OWASP
// CSV injection); tab and carriage return can hide one.
const formulaLead = "=+-@\t\r"

// safeCell neutralises text a spreadsheet would run as a formula by prefixing
// a single quote. Amounts are formatted by the server from decimals, so they
// are written as plain numbers (a negative amount stays a number).
func safeCell(s string) string {
	if s != "" && strings.ContainsRune(formulaLead, rune(s[0])) {
		return "'" + s
	}
	return s
}

// WriteCSV writes entries (as Export returns them) as CSV.
func WriteCSV(w io.Writer, currency string, list []Entry) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, e := range list {
		row := []string{
			e.At.UTC().Format(time.RFC3339),
			safeCell(deref(e.TripCode)),
			safeCell(e.Kind),
			safeCell(e.Memo),
			safeCell(deref(e.Holder)),
			safeCell(e.Reference),
			e.Amount.StringFixed(2),
			e.Running.StringFixed(2),
			safeCell(currency),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
