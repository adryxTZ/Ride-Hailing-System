package pdf

// Color is an RGB colour with each component in the range 0..1.
type Color struct {
	R, G, B float64
}

// Black is the default drawing colour of every page.
var Black = Color{}

// White is full-intensity RGB.
var White = Color{R: 1, G: 1, B: 1}

// RGB builds a colour from components in 0..1; out-of-range values are clamped.
func RGB(r, g, b float64) Color {
	return Color{R: clamp01(r), G: clamp01(g), B: clamp01(b)}
}

// Gray builds a neutral colour from a level in 0..1 (0 black, 1 white).
func Gray(level float64) Color {
	l := clamp01(level)
	return Color{R: l, G: l, B: l}
}

// clamp01 limits v to the closed interval [0, 1].
func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
