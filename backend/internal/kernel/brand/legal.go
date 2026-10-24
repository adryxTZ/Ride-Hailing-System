package brand

import (
	"fmt"
	"time"
)

// Legal holds the consent notices people accept (spec 2026-10-23-compliance-trust.md, S4).
// Since 2026-10-24-legal-documents.md the database is the source of truth: these
// only seed an app that has no consent version yet (the fallback without legal texts).
type Legal struct {
	Consent []ConsentDoc `json:"consent"`
}

// ConsentDoc is one version of an app's privacy notice and terms.
type ConsentDoc struct {
	App         string            `json:"app"` // RIDER | DRIVER
	Version     int               `json:"version"`
	Title       map[string]string `json:"title"`
	Summary     map[string]string `json:"summary"`
	URL         map[string]string `json:"url"`
	PublishedAt time.Time         `json:"published_at"`
}

// Privacy holds how privacy requests and data retention work for the brand.
type Privacy struct {
	// RequestDueDays is how long staff have to answer a request (default 30).
	RequestDueDays int `json:"request_due_days"`
	// Retention overrides the default days per retention rule (privacy package).
	Retention map[string]int `json:"retention"`
}

// DefaultRequestDueDays is the answer time when the brand file sets none.
const DefaultRequestDueDays = 30

// validateLegal fails fast on a consent list the apps could not show.
func (b *Brand) validateLegal() error {
	seen := map[string]bool{}
	for _, d := range b.Legal.Consent {
		if d.App != "RIDER" && d.App != "DRIVER" {
			return fmt.Errorf("brand %s: consent app %q must be RIDER or DRIVER", b.Code, d.App)
		}
		key := fmt.Sprintf("%s/%d", d.App, d.Version)
		if d.Version < 1 || seen[key] {
			return fmt.Errorf("brand %s: consent %s: version must be positive and listed once", b.Code, key)
		}
		seen[key] = true
		if d.Title[b.DefaultLocale] == "" || d.Summary[b.DefaultLocale] == "" || d.URL[b.DefaultLocale] == "" || d.PublishedAt.IsZero() {
			return fmt.Errorf("brand %s: consent %s needs title, summary and url in %s, and published_at", b.Code, key, b.DefaultLocale)
		}
	}
	if b.Privacy.RequestDueDays == 0 {
		b.Privacy.RequestDueDays = DefaultRequestDueDays
	}
	if b.Privacy.RequestDueDays < 1 {
		return fmt.Errorf("brand %s: privacy.request_due_days must be positive", b.Code)
	}
	for rule, days := range b.Privacy.Retention {
		if days < 1 {
			return fmt.Errorf("brand %s: privacy.retention.%s must be at least 1 day", b.Code, rule)
		}
	}
	return nil
}
