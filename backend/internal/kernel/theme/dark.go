package theme

import "fmt"

// Dark-mode derivation (DOC/03 §3.6): dark neutral surfaces tinted by the
// primary hue, the accent kept, light text. Staff can override any role.
const (
	darkBackgroundMix = 0.10 // share of primary mixed into the near-black background
	darkSurfaceMix    = 0.18 // a step lighter for cards
	darkBase          = "#0E1015"
)

// DeriveDark builds the dark palette from the light one, then applies overrides.
func DeriveDark(light Palette, overrides map[string]string) Palette {
	roles := map[string]string{
		"primary":    liftForDark(light.Primary),
		"secondary":  liftForDark(light.Secondary),
		"accent":     light.Accent,
		"background": mix(darkBase, light.Primary, darkBackgroundMix),
		"surface":    mix(darkBase, light.Primary, darkSurfaceMix),
		"success":    light.Success,
		"warning":    light.Warning,
		"danger":     light.Danger,
	}
	for k, v := range overrides {
		if v != "" {
			roles[k] = v
		}
	}
	return Derive(roles)
}

// liftForDark lightens a dark brand colour so it still reads on a dark page.
func liftForDark(hex string) string {
	if Contrast(hex, darkBase) >= MinTextContrast-1 {
		return hex
	}
	return mix(hex, "#FFFFFF", 0.35)
}

// mix blends b into a by share t (0…1) in sRGB.
func mix(a, b string, t float64) string {
	x, errA := parseHex(a)
	y, errB := parseHex(b)
	if errA != nil || errB != nil {
		return a
	}
	ch := func(i int) uint8 { return uint8(float64(x[i])*(1-t) + float64(y[i])*t + 0.5) }
	return fmt.Sprintf("#%02X%02X%02X", ch(0), ch(1), ch(2))
}
