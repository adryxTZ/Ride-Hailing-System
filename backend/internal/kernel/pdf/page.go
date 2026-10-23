package pdf

import (
	"bytes"
	"fmt"
)

// Page is one page of a Document. Drawing calls append operators to the
// page's content stream in call order. Coordinates are points from the
// top-left corner of the page, y growing downwards.
type Page struct {
	size    Size
	content bytes.Buffer
}

// Width returns the page width in points.
func (p *Page) Width() float64 { return p.size.Width }

// Height returns the page height in points.
func (p *Page) Height() float64 { return p.size.Height }

// SetColor sets the colour used by following Text and Line calls.
// Rect takes its own fill colour and does not change this one.
func (p *Page) SetColor(c Color) {
	fmt.Fprintf(&p.content, "%s %s %s rg %s %s %s RG\n",
		num(c.R), num(c.G), num(c.B), num(c.R), num(c.G), num(c.B))
}

// Text draws s with its left edge at x and its baseline at y.
func (p *Page) Text(x, y, size float64, font Font, s string) {
	p.text(x, y, size, font, toWinAnsi(s))
}

// TextRight draws s so that it ends at x, baseline at y (right-aligned).
func (p *Page) TextRight(x, y, size float64, font Font, s string) {
	encoded := toWinAnsi(s)
	p.text(x-encodedWidth(size, font, encoded), y, size, font, encoded)
}

// TextCenter draws s horizontally centred on x, baseline at y.
func (p *Page) TextCenter(x, y, size float64, font Font, s string) {
	encoded := toWinAnsi(s)
	p.text(x-encodedWidth(size, font, encoded)/2, y, size, font, encoded)
}

// text writes one BT/ET block for already-encoded bytes.
func (p *Page) text(x, y, size float64, font Font, encoded []byte) {
	fmt.Fprintf(&p.content, "BT /%s %s Tf %s %s Td (",
		font.resourceName(), num(size), num(x), num(p.flipY(y)))
	p.content.Write(escapeLiteral(encoded))
	p.content.WriteString(") Tj ET\n")
}

// Line strokes a straight line from (x1, y1) to (x2, y2) with the given
// width in points, using the colour set by SetColor.
func (p *Page) Line(x1, y1, x2, y2, width float64) {
	fmt.Fprintf(&p.content, "%s w %s %s m %s %s l S\n",
		num(width), num(x1), num(p.flipY(y1)), num(x2), num(p.flipY(y2)))
}

// Rect fills a w x h rectangle whose top-left corner is (x, y) with fill.
// The fill colour is scoped to this call (q/Q), so SetColor is unaffected.
func (p *Page) Rect(x, y, w, h float64, fill Color) {
	fmt.Fprintf(&p.content, "q %s %s %s rg %s %s %s %s re f Q\n",
		num(fill.R), num(fill.G), num(fill.B),
		num(x), num(p.flipY(y+h)), num(w), num(h))
}

// flipY converts a top-down y coordinate into PDF's bottom-up space.
func (p *Page) flipY(y float64) float64 {
	return p.size.Height - y
}
