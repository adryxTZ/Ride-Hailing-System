// Package i18n resolves user-facing messages by stable key and language.
// Error codes map to messages here; handlers never hard-code prose.
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Catalogue holds messages per language: lang → key → text.
type Catalogue struct {
	def   string
	langs map[string]map[string]string
}

// Load reads <dir>/<lang>.json for each language present. def is the fallback.
func Load(dir, def string) (*Catalogue, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	c := &Catalogue{def: def, langs: map[string]map[string]string{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		c.langs[strings.TrimSuffix(e.Name(), ".json")] = m
	}
	if _, ok := c.langs[def]; !ok {
		return nil, fmt.Errorf("default language %q not found in %s", def, dir)
	}
	return c, nil
}

// Pick chooses the best language from an Accept-Language header value.
func (c *Catalogue) Pick(acceptLanguage string) string {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if tag == "" {
			continue
		}
		if _, ok := c.langs[tag]; ok {
			return tag
		}
		if base := strings.SplitN(tag, "-", 2)[0]; base != tag {
			if _, ok := c.langs[base]; ok {
				return base
			}
		}
	}
	return c.def
}

// T returns the message for key in lang, falling back to the default language
// and finally to the key itself so a missing translation is visible, not fatal.
func (c *Catalogue) T(lang, key string) string {
	if m, ok := c.langs[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := c.langs[c.def][key]; ok {
		return s
	}
	return key
}
