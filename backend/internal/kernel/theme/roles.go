package theme

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// colorHuntPath matches a Color Hunt palette link: the four colours are the
// 24 hex characters in the URL itself, so nothing is fetched.
var colorHuntPath = regexp.MustCompile(`(?i)colorhunt\.co/palette/([0-9a-f]{24})\b`)

// ErrNotColorHunt is returned for a link that is not a Color Hunt palette.
var ErrNotColorHunt = fmt.Errorf("theme: not a colorhunt.co/palette/<24 hex> link")

// FromColorHunt reads the four colours of a Color Hunt palette link.
func FromColorHunt(link string) ([]string, error) {
	m := colorHuntPath.FindStringSubmatch(link)
	if m == nil {
		return nil, ErrNotColorHunt
	}
	hex := strings.ToUpper(m[1])
	out := make([]string, 0, 4)
	for i := 0; i < 24; i += 6 {
		out = append(out, "#"+hex[i:i+6])
	}
	return out, nil
}

// backgroundMinL: a palette's lightest colour must be at least this light (L*)
// to serve as the page background; otherwise the page is near-white and that
// colour becomes the surface tint.
const backgroundMinL = 85

// AssignRoles turns any four colours into roles (DOC/03 §3): the lightest is
// the background, the next the surface, the more colourful of the other two
// the accent and the remaining one the primary.
func AssignRoles(colours []string) map[string]string {
	sorted := append([]string(nil), colours...)
	sort.SliceStable(sorted, func(i, j int) bool { return toLab(sorted[i]).L > toLab(sorted[j]).L })
	roles := map[string]string{}
	if len(sorted) < 4 {
		if len(sorted) > 0 {
			roles["primary"] = sorted[len(sorted)-1]
		}
		return roles
	}
	bg, surface, a, b := sorted[0], sorted[1], sorted[2], sorted[3]
	if toLab(bg).L < backgroundMinL {
		bg, surface = "#FFFFFF", bg
	}
	accent, primary := a, b
	if toLab(b).chroma() > toLab(a).chroma() {
		accent, primary = b, a
	}
	roles["background"], roles["surface"], roles["accent"], roles["primary"] = bg, surface, accent, primary
	roles["secondary"] = primary
	return roles
}
