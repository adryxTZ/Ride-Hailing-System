package legal

import (
	"strconv"
	"time"
)

// monthNames spell the effective date in each language the texts ship in.
var monthNames = map[string][12]string{
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	"sw": {"Januari", "Februari", "Machi", "Aprili", "Mei", "Juni", "Julai", "Agosti", "Septemba", "Oktoba", "Novemba", "Desemba"},
}

// longDate is "24 October 2026" (English month names for a language without its own).
func longDate(t time.Time, lang string) string {
	names, ok := monthNames[lang]
	if !ok {
		names = monthNames["en"]
	}
	return strconv.Itoa(t.Day()) + " " + names[t.Month()-1] + " " + strconv.Itoa(t.Year())
}
