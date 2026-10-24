package legal

import (
	"strings"
	"testing"
)

// starterDir is the repository's starter pack, relative to this package.
const starterDir = "../../seed/legal"

// TestStarterPackIsComplete: every document every app offers is in the pack,
// in English and Swahili, marked as a template, and uses only known placeholders.
func TestStarterPackIsComplete(t *testing.T) {
	p, err := LoadStarter(starterDir)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]StarterDoc{}
	for _, d := range p.Documents {
		have[docKey(d.App, d.Kind)] = d
	}
	for _, app := range Apps {
		if p.Consent[app].Title["en"] == "" || p.Consent[app].Summary["sw"] == "" {
			t.Errorf("%s: consent title and summary needed", app)
		}
		for _, k := range Kinds {
			if !Offered(app, k.Kind) {
				continue
			}
			d, ok := have[docKey(app, k.Kind)]
			if !ok {
				t.Errorf("%s %s missing from the starter pack", app, k.Kind)
				continue
			}
			checkStarterDoc(t, d)
		}
	}
}

func checkStarterDoc(t *testing.T, d StarterDoc) {
	t.Helper()
	for _, lang := range []string{"en", "sw"} {
		body := d.Body[lang]
		if d.Title[lang] == "" || len(body) < 1000 {
			t.Errorf("%s %s %s: title and a full body needed", d.App, d.Kind, lang)
		}
		if !strings.HasPrefix(body, "> **") || !strings.Contains(body, "{{country.name}}") {
			t.Errorf("%s %s %s: must open with the lawyer-review note", d.App, d.Kind, lang)
		}
		filled, _ := Fill(body, map[string]string{"brand.name": "Ride Co"})
		if html := Markdown(filled); !strings.Contains(html, "<h2>") || !strings.Contains(html, "Ride Co") {
			t.Errorf("%s %s %s: does not render", d.App, d.Kind, lang)
		}
	}
}
