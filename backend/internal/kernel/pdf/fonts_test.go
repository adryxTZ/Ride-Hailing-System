package pdf

import (
	"math"
	"testing"
)

// widthTolerance absorbs float rounding in width comparisons.
const widthTolerance = 1e-9

func TestTextWidth(t *testing.T) {
	cases := []struct {
		font Font
		s    string
		size float64
		want float64
	}{
		{Regular, "Hello", 10, 22.78},
		{Bold, "Hello", 10, 24.45},
		{Regular, "", 10, 0},
		{Regular, "€", 10, 5.56},
		{Regular, "ő", 10, 5.56}, // counts as '?'
	}
	for _, c := range cases {
		if got := TextWidth(c.size, c.font, c.s); math.Abs(got-c.want) > widthTolerance {
			t.Errorf("TextWidth(%v, %q) = %v, want %v", c.font, c.s, got, c.want)
		}
	}
}

func TestBoldIsWiderForLowercase(t *testing.T) {
	s := "abcdefghijklmnopqrstuvwxyz"
	if TextWidth(10, Bold, s) <= TextWidth(10, Regular, s) {
		t.Fatal("Helvetica-Bold should be wider than Helvetica")
	}
}

func TestTextRightEndsAtX(t *testing.T) {
	doc := New(A4)
	p := doc.AddPage()
	p.TextRight(100, 50, 10, Regular, "Hello")
	// left edge = 100 - 22.78
	if got := p.content.String(); got != "BT /F1 10 Tf 77.22 791.89 Td (Hello) Tj ET\n" {
		t.Fatalf("content = %q", got)
	}
}
