package legal

import (
	"regexp"
	"sort"
	"strings"
)

// Where a placeholder's value comes from.
const (
	FromBrand    = "BRAND"    // the Branding page (identity) or the brand file
	FromCountry  = "COUNTRY"  // the brand's home country
	FromDetails  = "DETAILS"  // the Legal details form on the Legal page
	FromDocument = "DOCUMENT" // the version being shown
)

// Placeholder is one {{key}} the texts may use.
type Placeholder struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Hint   string `json:"hint,omitempty"`
	Source string `json:"source"`
}

// Placeholders is the catalogue; only these keys may appear in a text.
var Placeholders = []Placeholder{
	{"brand.name", "Brand name", "From the Branding page.", FromBrand},
	{"legal_entity", "Legal entity", "The company that runs the service. Empty: the legal entity on the Branding page.", FromDetails},
	{"registered_address", "Registered address", "The company's registered office.", FromDetails},
	{"company_number", "Company registration number", "", FromDetails},
	{"dpo_name", "Data protection officer", "The person or team people can write to about their data.", FromDetails},
	{"dpo_email", "Data protection e-mail", "Where people send privacy questions and requests.", FromDetails},
	{"data_protection_authority", "Data protection authority", "The national authority people may complain to.", FromDetails},
	{"regulator", "Transport regulator", "Empty: the regulator named in the brand file.", FromDetails},
	{"minimum_age", "Minimum age", "The age a person must be to hold an account. Empty: 18.", FromDetails},
	{"support_email", "Support e-mail", "From the Branding page.", FromBrand},
	{"support_phone", "Support phone", "From the Branding page.", FromBrand},
	{"emergency_phone", "Emergency number", "From the Branding page.", FromBrand},
	{"website", "Website", "The brand's web domain (brand file).", FromBrand},
	{"country.name", "Country", "The brand's home country.", FromCountry},
	{"effective_date", "Effective date", "The day the version is published.", FromDocument},
	{"version", "Version", "The version number.", FromDocument},
}

// defaultMinimumAge fills {{minimum_age}} when staff set none.
const defaultMinimumAge = "18"

var placeholderRe = regexp.MustCompile(`\{\{\s*([A-Za-z][A-Za-z0-9_.]*)\s*\}\}`)

// knownPlaceholder reports whether a key is in the catalogue.
func knownPlaceholder(key string) bool {
	for _, p := range Placeholders {
		if p.Key == key {
			return true
		}
	}
	return false
}

// editable reports whether staff fill this key on the Legal details form.
func editable(key string) bool {
	for _, p := range Placeholders {
		if p.Key == key {
			return p.Source == FromDetails
		}
	}
	return false
}

// UnknownPlaceholders lists the keys in a text that are not in the catalogue.
func UnknownPlaceholders(src string) []string {
	seen := map[string]bool{}
	for _, m := range placeholderRe.FindAllStringSubmatch(src, -1) {
		if !knownPlaceholder(m[1]) {
			seen[m[1]] = true
		}
	}
	return sortedKeys(seen)
}

// Fill replaces each {{key}} with its value (escaped so a value can never add
// Markdown); a known key without a value shows as [key] and is reported missing.
func Fill(src string, values map[string]string) (out string, missing []string) {
	gaps := map[string]bool{}
	out = placeholderRe.ReplaceAllStringFunc(src, func(m string) string {
		key := placeholderRe.FindStringSubmatch(m)[1]
		if !knownPlaceholder(key) {
			return escapeMarkdown(m)
		}
		if v := strings.TrimSpace(values[key]); v != "" {
			return escapeMarkdown(v)
		}
		gaps[key] = true
		return `\[` + key + `\]`
	})
	return out, sortedKeys(gaps)
}

// escapeMarkdown backslash-escapes the characters the renderer treats as markers.
func escapeMarkdown(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("\\`*[]#", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
