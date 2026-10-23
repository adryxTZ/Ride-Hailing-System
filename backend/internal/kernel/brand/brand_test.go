package brand

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBrand(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	raw := `{"code":"xb","display_name":"X","country":"TZ","currency":"TZS","timezone":"Africa/Dar_es_Salaam","default_locale":"en","theme_preset":"p"` + extra + `}`
	if err := os.WriteFile(filepath.Join(dir, "xb.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLegacyBrandBuildsOneCountry(t *testing.T) {
	b, err := Load(writeBrand(t, ""), "xb")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Countries) != 1 || b.Countries[0].Code != "TZ" || b.Countries[0].Currency != "TZS" || b.Countries[0].Status != CountryActive {
		t.Fatalf("legacy brand should get one ACTIVE home country: %+v", b.Countries)
	}
}

func TestCountryValidation(t *testing.T) {
	cases := map[string]string{
		"home country missing": `,"countries":[{"code":"KE","currency":"KES","timezone":"Africa/Nairobi","status":"ACTIVE"}]`,
		"duplicate":            `,"countries":[{"code":"TZ","currency":"TZS","timezone":"Africa/Dar_es_Salaam","status":"ACTIVE"},{"code":"TZ","currency":"TZS","timezone":"Africa/Dar_es_Salaam"}]`,
		"bad currency":         `,"countries":[{"code":"TZ","currency":"tzs","timezone":"Africa/Dar_es_Salaam","status":"ACTIVE"}]`,
		"bad timezone":         `,"countries":[{"code":"TZ","currency":"TZS","timezone":"Mars/Olympus","status":"ACTIVE"}]`,
		"nothing active":       `,"countries":[{"code":"TZ","currency":"TZS","timezone":"Africa/Dar_es_Salaam","status":"DRAFT"}]`,
	}
	for name, extra := range cases {
		if _, err := Load(writeBrand(t, extra), "xb"); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestShippedBrandsLoad(t *testing.T) {
	files, err := filepath.Glob("../../../brands/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no brand files: %v", err)
	}
	for _, f := range files {
		code := strings.TrimSuffix(filepath.Base(f), ".json")
		b, err := Load("../../../brands", code)
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if b.CountryByCode(b.Country) == nil {
			t.Fatalf("%s: home country missing", code)
		}
		var js json.RawMessage
		if raw, _ := json.Marshal(b.Countries); !strings.Contains(string(raw), `"TZ"`) || json.Unmarshal(raw, &js) != nil {
			t.Fatalf("%s: countries not serialisable", code)
		}
	}
}
