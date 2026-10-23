package pdf

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// numberDecimals is the precision of coordinates and sizes in content
// streams; 1/1000 pt is far below what any device can render.
const numberDecimals = 3

// pdfDateLayout formats a UTC time as a PDF date string body.
const pdfDateLayout = "20060102150405"

// utf16BOM marks a PDF text string as UTF-16BE.
const utf16BOM = "FEFF"

// num formats v with fixed precision and no trailing zeros, so the same
// value always produces the same bytes (never exponent notation).
func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', numberDecimals, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

// pdfDate renders t as a PDF date "D:YYYYMMDDHHmmSSZ" in UTC.
func pdfDate(t time.Time) string {
	return "D:" + t.UTC().Format(pdfDateLayout) + "Z"
}

// textString renders s as a hex PDF text string in UTF-16BE with a BOM.
// Why UTF-16: Info strings use PDFDocEncoding, not WinAnsi, and UTF-16 is
// exact for every rune without a second code table.
func textString(s string) string {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, 0, len(units)*2)
	for _, u := range units {
		buf = append(buf, byte(u>>8), byte(u))
	}
	return "<" + utf16BOM + strings.ToUpper(hex.EncodeToString(buf)) + ">"
}
