// Package brand loads a brand definition. A brand is data: the Platform never
// hard-codes a product name, colour or logo.
package brand

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"
)

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)

type Brand struct {
	Code          string            `json:"code"`
	DisplayName   string            `json:"display_name"`
	LegalEntity   string            `json:"legal_entity"`
	Country       string            `json:"country"`
	Currency      string            `json:"currency"`
	Timezone      string            `json:"timezone"`
	DefaultLocale string            `json:"default_locale"`
	Locales       []string          `json:"locales"`
	Domains       map[string]string `json:"domains"`
	Apps          map[string]App    `json:"apps"`
	ThemePreset   string            `json:"theme_preset"`
	Providers     map[string]string `json:"providers"`
	Services      []string          `json:"services_enabled"`
	SupportPhone  string            `json:"support_phone"`
	SupportEmail  string            `json:"support_email"`
	// EmergencyPhone is the country's emergency number the apps offer in Safety (e.g. 112).
	EmergencyPhone string `json:"emergency_phone"`
	// Countries the brand operates in. Country, Currency and Timezone above are the
	// home country; a file without this list gets one entry built from them.
	Countries []Country `json:"countries"`
	// Legal is the privacy notice and terms per app and version; Privacy the request and retention rules.
	Legal   Legal   `json:"legal"`
	Privacy Privacy `json:"privacy"`
}

// Country is one country a brand operates in, with its own money and rules.
type Country struct {
	Code           string            `json:"code"`      // ISO 3166-1 alpha-2
	Name           map[string]string `json:"name"`      // by locale
	Currency       string            `json:"currency"`  // ISO 4217
	Timezone       string            `json:"timezone"`  // IANA
	AreaTerm       map[string]string `json:"area_term"` // what the country calls its areas (Region, County, State), by locale
	Regulator      string            `json:"regulator"`
	PaymentGateway string            `json:"payment_gateway"`
	EmergencyPhone string            `json:"emergency_phone"`
	Map            MapView           `json:"map"`
	Status         string            `json:"status"` // ACTIVE or DRAFT
	// LicencePattern is the regular expression a driving licence number must
	// match (e.g. ^\d{10}$ in Tanzania); empty = any.
	LicencePattern string `json:"licence_pattern,omitempty"`
	// LicenceClasses are the licence categories drivers choose from.
	LicenceClasses []string `json:"licence_classes,omitempty"`
}

// LicenceOK reports whether a licence number matches the country's pattern.
func (c *Country) LicenceOK(number string) bool {
	if c.LicencePattern == "" {
		return true
	}
	re, err := regexp.Compile(c.LicencePattern)
	return err == nil && re.MatchString(number)
}

// LicenceClassOK reports whether a class is one the country lists (any when none are listed).
func (c *Country) LicenceClassOK(class string) bool {
	if len(c.LicenceClasses) == 0 {
		return true
	}
	return slices.Contains(c.LicenceClasses, class)
}

// MapView is where the admin map opens for a country.
type MapView struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Zoom int     `json:"zoom"`
}

// Country statuses.
const (
	CountryActive = "ACTIVE"
	CountryDraft  = "DRAFT"
)

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

// validateCountries fills a legacy single-country brand and checks the list.
func (b *Brand) validateCountries() error {
	if len(b.Countries) == 0 {
		b.Countries = []Country{{Code: b.Country, Name: map[string]string{"en": b.Country}, Currency: b.Currency, Timezone: b.Timezone,
			AreaTerm: map[string]string{"en": "Region", "sw": "Mkoa"}, Regulator: b.Providers["regulator"], PaymentGateway: b.Providers["payment_gateway"],
			EmergencyPhone: b.EmergencyPhone, Status: CountryActive}}
	}
	seen := map[string]bool{}
	active, home := 0, false
	for i := range b.Countries {
		c := &b.Countries[i]
		if seen[c.Code] {
			return fmt.Errorf("brand %s: country %s listed twice", b.Code, c.Code)
		}
		seen[c.Code] = true
		if !currencyRe.MatchString(c.Currency) {
			return fmt.Errorf("brand %s: country %s has an invalid currency %q", b.Code, c.Code, c.Currency)
		}
		if _, err := time.LoadLocation(c.Timezone); err != nil || c.Timezone == "" {
			return fmt.Errorf("brand %s: country %s has an invalid timezone %q", b.Code, c.Code, c.Timezone)
		}
		if c.Status == "" {
			c.Status = CountryDraft
		}
		if _, err := regexp.Compile(c.LicencePattern); err != nil {
			return fmt.Errorf("brand %s: country %s has an invalid licence_pattern: %w", b.Code, c.Code, err)
		}
		if c.Status != CountryActive && c.Status != CountryDraft {
			return fmt.Errorf("brand %s: country %s has an invalid status %q", b.Code, c.Code, c.Status)
		}
		if c.Status == CountryActive {
			active++
		}
		if c.Code == b.Country {
			home = true
			if c.Currency != b.Currency || c.Timezone != b.Timezone {
				return fmt.Errorf("brand %s: home country %s must match the brand currency and timezone", b.Code, c.Code)
			}
		}
	}
	if !home {
		return fmt.Errorf("brand %s: home country %s is not in countries", b.Code, b.Country)
	}
	if active == 0 {
		return fmt.Errorf("brand %s: no ACTIVE country", b.Code)
	}
	return nil
}

// CountryByCode returns the brand's country, or nil.
func (b *Brand) CountryByCode(code string) *Country {
	for i := range b.Countries {
		if b.Countries[i].Code == code {
			return &b.Countries[i]
		}
	}
	return nil
}

type App struct {
	Slug           string `json:"slug"`
	AndroidPackage string `json:"android_package"`
	IOSBundleID    string `json:"ios_bundle_id"`
}

// Load reads <dir>/<code>.json and validates the fields the runtime relies on.
func Load(dir, code string) (*Brand, error) {
	if !codeRe.MatchString(code) {
		return nil, fmt.Errorf("invalid brand code %q", code)
	}
	raw, err := os.ReadFile(filepath.Join(dir, code+".json"))
	if err != nil {
		return nil, err
	}
	var b Brand
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("parse brand %s: %w", code, err)
	}
	if b.Code != code {
		return nil, fmt.Errorf("brand file %s declares code %q", code, b.Code)
	}
	for k, v := range map[string]string{"display_name": b.DisplayName, "country": b.Country, "currency": b.Currency, "default_locale": b.DefaultLocale, "theme_preset": b.ThemePreset} {
		if v == "" {
			return nil, fmt.Errorf("brand %s: %s is required", code, k)
		}
	}
	if err := b.validateCountries(); err != nil {
		return nil, err
	}
	if err := b.validateLegal(); err != nil {
		return nil, err
	}
	return &b, nil
}
