package legal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// starterManifest is seed/legal/manifest.json: the starter pack's documents
// (their Markdown files sit beside it) and each app's consent screen texts.
const starterManifest = "manifest.json"

// StarterPack is the generic texts every new brand starts with: templates to
// be reviewed by a lawyer, with placeholders filled from brand data.
type StarterPack struct {
	Consent   map[string]StarterConsent `json:"consent"`
	Documents []StarterDoc              `json:"documents"`
}

// StarterConsent is an app's consent screen heading and short text.
type StarterConsent struct {
	Title   Text `json:"title"`
	Summary Text `json:"summary"`
}

// StarterDoc is one starter text: its titles and its Markdown file per language.
type StarterDoc struct {
	App     string            `json:"app"`
	Kind    string            `json:"kind"`
	Title   Text              `json:"title"`
	Summary Text              `json:"summary"`
	Files   map[string]string `json:"files"`
	Body    Text              `json:"-"`
}

// LoadStarter reads the starter pack from a directory and checks it: known
// apps and kinds, every file present, and only catalogue placeholders.
func LoadStarter(dir string) (*StarterPack, error) {
	raw, err := os.ReadFile(filepath.Join(dir, starterManifest))
	if err != nil {
		return nil, err
	}
	var p StarterPack
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", starterManifest, err)
	}
	for i := range p.Documents {
		if err := p.Documents[i].load(dir); err != nil {
			return nil, err
		}
	}
	return &p, nil
}

func (d *StarterDoc) load(dir string) error {
	if !Offered(d.App, d.Kind) {
		return fmt.Errorf("starter: %s has no %s", d.App, d.Kind)
	}
	d.Body = Text{}
	for lang, name := range d.Files {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.Clean(name)))
		if err != nil {
			return fmt.Errorf("starter %s/%s: %w", d.App, d.Kind, err)
		}
		if unknown := UnknownPlaceholders(string(raw)); len(unknown) > 0 {
			return fmt.Errorf("starter %s: unknown placeholders %v", name, unknown)
		}
		d.Body[lang] = string(raw)
	}
	return nil
}

// SeedStarter publishes the starter pack as version 1 of every document when
// the brand has none yet (an empty database or a new brand) and links each
// app's consent to it. Returns how many texts it added; idempotent.
func (s *Service) SeedStarter(ctx context.Context, p *StarterPack) (int, error) {
	n := 0
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.lock(ctx, tx, "starter"); err != nil {
			return err
		}
		if have, err := s.store.count(ctx, tx); err != nil || have > 0 {
			return err
		}
		for _, d := range p.Documents {
			if err := s.insertStarter(ctx, tx, d); err != nil {
				return err
			}
			n++
		}
		if err := s.seedConsent(ctx, tx, p); err != nil {
			return err
		}
		return s.store.bump(ctx, tx)
	})
	return n, err
}

func (s *Service) insertStarter(ctx context.Context, tx pgx.Tx, d StarterDoc) error {
	doc := Document{App: d.App, Kind: d.Kind, Title: s.inLocales(d.Title), Summary: s.inLocales(d.Summary), Body: s.inLocales(d.Body)}
	id, err := s.store.insertStarter(ctx, tx, doc)
	if err != nil {
		return fmt.Errorf("starter %s/%s: %w", d.App, d.Kind, err)
	}
	return audit.Record(ctx, tx, s.store.brandID, "", "legal.starter", "legal:"+id, starterReason, map[string]any{"app": d.App, "kind": d.Kind})
}

// seedConsent links each app's consent to the starter texts.
func (s *Service) seedConsent(ctx context.Context, tx pgx.Tx, p *StarterPack) error {
	if s.d.Consent == nil {
		return nil
	}
	for _, app := range Apps {
		c := p.Consent[app]
		v, err := s.consentVersion(ctx, tx, app, s.inLocales(c.Title))
		if err != nil {
			return err
		}
		if len(v.Links) == 0 {
			continue
		}
		if len(v.Summary) == 0 {
			v.Summary = s.inLocales(c.Summary)
		}
		if _, err := s.d.Consent.SeedVersionTx(ctx, tx, v); err != nil {
			return err
		}
	}
	return nil
}

// inLocales keeps the languages the brand speaks.
func (s *Service) inLocales(t Text) Text {
	out := Text{}
	for lang, v := range t {
		if slices.Contains(s.d.Brand.Locales, lang) {
			out[lang] = v
		}
	}
	return out
}
