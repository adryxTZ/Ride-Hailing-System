package pdf

// Font selects one of the built-in fonts this package can draw with.
type Font int

const (
	// Regular is Helvetica.
	Regular Font = iota
	// Bold is Helvetica-Bold.
	Bold
)

// glyphUnitsPerEm is the AFM unit scale: widths are thousandths of the font size.
const glyphUnitsPerEm = 1000

// baseFontName returns the PDF /BaseFont name of f.
func (f Font) baseFontName() string {
	if f == Bold {
		return "Helvetica-Bold"
	}
	return "Helvetica"
}

// resourceName returns the page resource name (without the slash) used for f.
func (f Font) resourceName() string {
	if f == Bold {
		return "F2"
	}
	return "F1"
}

// widths returns the WinAnsi width table for f.
func (f Font) widths() *[winAnsiGlyphCount]uint16 {
	if f == Bold {
		return &helveticaBoldWidths
	}
	return &helveticaWidths
}

// TextWidth returns the advance width in points of s drawn at size in font,
// measured after WinAnsi conversion (so unencodable runes count as '?').
func TextWidth(size float64, font Font, s string) float64 {
	return encodedWidth(size, font, toWinAnsi(s))
}

// encodedWidth measures already-encoded WinAnsi bytes.
func encodedWidth(size float64, font Font, b []byte) float64 {
	table := font.widths()
	units := 0
	for _, c := range b {
		units += int(glyphWidth(table, c))
	}
	return float64(units) * size / glyphUnitsPerEm
}

// glyphWidth looks up one code; codes below the table start are zero width.
func glyphWidth(table *[winAnsiGlyphCount]uint16, c byte) uint16 {
	if c < winAnsiFirstChar {
		return 0
	}
	return table[c-winAnsiFirstChar]
}
