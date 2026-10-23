package theme

import "math"

// Colour maths shared by role assignment, contrast and distinctness checks.

// lab is a CIE L*a*b* colour (D65), used for perceptual lightness and ΔE.
type lab struct{ L, A, B float64 }

// srgbToLinear undoes the sRGB transfer curve for one 0–255 channel.
func srgbToLinear(c uint8) float64 {
	v := float64(c) / 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// toLab converts a hex colour to L*a*b*; invalid input is black.
func toLab(hex string) lab {
	rgb, err := parseHex(hex)
	if err != nil {
		return lab{}
	}
	r, g, b := srgbToLinear(rgb[0]), srgbToLinear(rgb[1]), srgbToLinear(rgb[2])
	x := (0.4124*r + 0.3576*g + 0.1805*b) / 0.95047
	y := 0.2126*r + 0.7152*g + 0.0722*b
	z := (0.0193*r + 0.1192*g + 0.9505*b) / 1.08883
	f := func(t float64) float64 {
		const eps = 216.0 / 24389
		if t > eps {
			return math.Cbrt(t)
		}
		return (24389.0/27*t + 16) / 116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return lab{L: 116*fy - 16, A: 500 * (fx - fy), B: 200 * (fy - fz)}
}

// chroma is how colourful a colour is (0 = grey).
func (c lab) chroma() float64 { return math.Hypot(c.A, c.B) }

// DeltaE is the CIE76 distance between two colours (under ~10 looks alike).
func DeltaE(a, b string) float64 {
	x, y := toLab(a), toLab(b)
	return math.Sqrt((x.L-y.L)*(x.L-y.L) + (x.A-y.A)*(x.A-y.A) + (x.B-y.B)*(x.B-y.B))
}

// ValidHex reports whether s is a #RRGGBB colour.
func ValidHex(s string) bool {
	_, err := parseHex(s)
	return err == nil && len(s) == 7 && s[0] == '#'
}
