// Package theme turns a brand's palette choice into the full set of design
// tokens the apps and admin render with. Presets are data (seed/theme_presets.json);
// derivation (text-on-colour, semantic defaults) lives here so every brand gets
// an accessible result without a designer.
//
//   - theme.go   Palette, Theme, presets, Derive, Contrast
//   - roles.go   Color Hunt links and automatic role assignment for four colours
//   - dark.go    dark-mode derivation
//   - report.go  the publish rules (contrast, primary vs accent)
//   - colour.go  L*a*b*, ΔE and hex validation
package theme

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Palette struct {
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Accent     string `json:"accent"`
	Background string `json:"background"`
	Surface    string `json:"surface"`
	Text       string `json:"text"`
	Muted      string `json:"muted"`
	Border     string `json:"border"`
	Success    string `json:"success"`
	Warning    string `json:"warning"`
	Danger     string `json:"danger"`
	Info       string `json:"info"`
	OnPrimary  string `json:"on_primary"`
	OnAccent   string `json:"on_accent"`
	Highlight  string `json:"highlight"` // tint for tiles and chips; falls back to surface
}

// Theme is what /app/config serves: the published version's tokens.
type Theme struct {
	Version    int               `json:"version"`
	Preset     string            `json:"preset,omitempty"`
	Palette    Palette           `json:"palette"`
	Dark       *Palette          `json:"dark,omitempty"`
	Typography map[string]string `json:"typography"`
	Shape      map[string]int    `json:"shape,omitempty"`
	// Assets maps a slot (logo_full, logo_mark, app_icon …) to a public URL path.
	Assets map[string]string `json:"assets"`
}

type Preset struct {
	Key       string            `json:"key"`
	Name      string            `json:"name"`
	Colours   []string          `json:"colours"`
	Roles     map[string]string `json:"roles"`
	SourceURL *string           `json:"source_url"`
	Tags      []string          `json:"tags"`
}

type Presets struct {
	byKey map[string]Preset
	order []string
}

func LoadPresets(dir string) (*Presets, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "theme_presets.json"))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Presets []Preset `json:"presets"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	p := &Presets{byKey: map[string]Preset{}}
	for _, pr := range doc.Presets {
		if len(pr.Colours) != 4 {
			return nil, fmt.Errorf("preset %q must have exactly 4 colours", pr.Key)
		}
		for _, c := range pr.Colours {
			if _, err := parseHex(c); err != nil {
				return nil, fmt.Errorf("preset %q: %w", pr.Key, err)
			}
		}
		p.byKey[pr.Key] = pr
		p.order = append(p.order, pr.Key)
	}
	return p, nil
}

func (p *Presets) List() []Preset {
	out := make([]Preset, 0, len(p.order))
	for _, k := range p.order {
		out = append(out, p.byKey[k])
	}
	return out
}

// Get returns one preset by key.
func (p *Presets) Get(key string) (Preset, bool) {
	pr, ok := p.byKey[key]
	return pr, ok
}

// Resolve builds a complete Theme from a preset key.
func (p *Presets) Resolve(key string) (*Theme, bool) {
	pr, ok := p.byKey[key]
	if !ok {
		return nil, false
	}
	pal := Derive(pr.Roles)
	return &Theme{
		Version:    1,
		Preset:     key,
		Palette:    pal,
		Typography: map[string]string{"heading_font": "Plus Jakarta Sans", "body_font": "Figtree"},
		Assets:     map[string]string{},
	}, true
}

// Derive fills every token from the roles a palette provides. Missing roles get
// accessible defaults; text-on-colour is chosen by WCAG contrast.
func Derive(roles map[string]string) Palette {
	get := func(k, def string) string {
		if v := roles[k]; v != "" {
			return strings.ToUpper(v)
		}
		return def
	}
	pal := Palette{
		Primary:    get("primary", "#0B2447"),
		Secondary:  get("secondary", get("primary", "#19376D")),
		Accent:     get("accent", "#576CBC"),
		Background: get("background", "#FFFFFF"),
		Surface:    get("surface", "#F4F6FA"),
		Success:    get("success", "#1B9C5A"),
		Warning:    get("warning", "#E0A100"),
		Danger:     get("danger", "#D64545"),
	}
	dark := relLum(pal.Background) < 0.2
	if dark {
		pal.Text = get("text", "#F2F4F7")
		pal.Muted = get("muted", "#A9B1BD")
		pal.Border = get("border", "#2E3640")
	} else {
		pal.Text = get("text", "#111318")
		pal.Muted = get("muted", "#5F6B7A")
		pal.Border = get("border", "#D9DEE5")
	}
	pal.Info = get("info", pal.Primary)
	pal.Highlight = get("highlight", pal.Surface)
	pal.OnPrimary = onColour(pal.Primary)
	pal.OnAccent = onColour(pal.Accent)
	return pal
}

// Contrast returns the WCAG contrast ratio between two hex colours.
func Contrast(a, b string) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func onColour(bg string) string {
	if Contrast(bg, "#FFFFFF") >= Contrast(bg, "#111318") {
		return "#FFFFFF"
	}
	return "#111318"
}

func relLum(hex string) float64 {
	rgb, err := parseHex(hex)
	if err != nil {
		return 0
	}
	lin := func(c uint8) float64 {
		v := float64(c) / 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(rgb[0]) + 0.7152*lin(rgb[1]) + 0.0722*lin(rgb[2])
}

func parseHex(s string) ([3]uint8, error) {
	var out [3]uint8
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return out, fmt.Errorf("invalid hex colour %q", s)
	}
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseUint(s[i*2:i*2+2], 16, 8)
		if err != nil {
			return out, fmt.Errorf("invalid hex colour %q", s)
		}
		out[i] = uint8(v)
	}
	return out, nil
}
