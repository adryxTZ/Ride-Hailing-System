package theme

// Publishing rules (DOC/03 §7): readable text and buttons, and a primary and
// accent people can tell apart.
const (
	MinTextContrast = 4.5
	MinDistinctness = 10 // ΔE between primary and accent
)

// Check is one rule of the contrast report.
type Check struct {
	Key   string  `json:"key"` // TEXT_ON_BACKGROUND, TEXT_ON_SURFACE, ON_PRIMARY, ON_ACCENT, PRIMARY_VS_ACCENT
	Value float64 `json:"value"`
	Min   float64 `json:"min"`
	Pass  bool    `json:"pass"`
}

// Report is the result of every rule; Pass is true only when all pass.
type Report struct {
	Pass   bool    `json:"pass"`
	Checks []Check `json:"checks"`
}

// Evaluate checks a palette (and its dark variant when given) against the rules.
func Evaluate(light Palette, dark *Palette) Report {
	checks := paletteChecks("", light)
	if dark != nil {
		checks = append(checks, paletteChecks("DARK_", *dark)...)
	}
	r := Report{Pass: true, Checks: checks}
	for _, c := range checks {
		r.Pass = r.Pass && c.Pass
	}
	return r
}

func paletteChecks(prefix string, p Palette) []Check {
	ratio := func(key string, a, b string) Check {
		v := Contrast(a, b)
		return Check{Key: prefix + key, Value: round2(v), Min: MinTextContrast, Pass: v >= MinTextContrast}
	}
	d := DeltaE(p.Primary, p.Accent)
	return []Check{
		ratio("TEXT_ON_BACKGROUND", p.Text, p.Background),
		ratio("TEXT_ON_SURFACE", p.Text, p.Surface),
		ratio("ON_PRIMARY", p.OnPrimary, p.Primary),
		ratio("ON_ACCENT", p.OnAccent, p.Accent),
		{Key: prefix + "PRIMARY_VS_ACCENT", Value: round2(d), Min: MinDistinctness, Pass: d >= MinDistinctness},
	}
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
