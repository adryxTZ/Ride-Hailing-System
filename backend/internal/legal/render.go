package legal

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"
)

// regulatorNone is the brand file's "this country has no transport regulator".
const regulatorNone = "none"

// Page renders a document's live version in a language (the brand's default
// language when it has no text in the one asked for). Reads the cache only.
func (s *Service) Page(ctx context.Context, app, kind, lang string) (Page, error) {
	snap, _, err := s.cache.Get(ctx)
	if err != nil {
		return Page{}, err
	}
	d := snap.live[docKey(app, kind)]
	if d == nil {
		return Page{}, ErrNotFound
	}
	p, _, err := s.render(ctx, d, lang, snap.details)
	return p, err
}

// Preview renders an unsaved text as the next version would read on its effective date (today).
func (s *Service) Preview(ctx context.Context, app, kind, lang string, in DraftInput) (Preview, error) {
	if !Offered(app, kind) {
		return Preview{}, ErrNotFound
	}
	if strings.TrimSpace(in.Body[lang]) == "" {
		return Preview{Missing: []string{}, Unknown: []string{}}, nil // nothing written in this language yet
	}
	snap, _, err := s.cache.Get(ctx)
	if err != nil {
		return Preview{}, err
	}
	next := 1
	if live := snap.live[docKey(app, kind)]; live != nil {
		next = live.Version + 1
	}
	d := &Document{App: app, Kind: kind, Version: next, Title: in.Title, Body: in.Body}
	p, missing, err := s.render(ctx, d, lang, snap.details)
	if err != nil {
		return Preview{}, err
	}
	unknown := UnknownPlaceholders(pick(in.Title, p.Lang) + "\n" + pick(in.Body, p.Lang))
	return Preview{HTML: p.HTML, Missing: missing, Unknown: unknown}, nil
}

// render fills the placeholders and turns the Markdown into sanitised HTML.
func (s *Service) render(ctx context.Context, d *Document, lang string, details map[string]string) (Page, []string, error) {
	lang = s.textLang(d.Body, lang)
	vals, err := s.values(ctx, details, lang, d)
	if err != nil {
		return Page{}, nil, err
	}
	body, missing := Fill(pick(d.Body, lang, s.d.Brand.DefaultLocale), vals)
	p := Page{App: d.App, Kind: d.Kind, Version: d.Version, Lang: lang, HTML: Markdown(body),
		Title: fillPlain(pick(d.Title, lang, s.d.Brand.DefaultLocale), vals), EffectiveDate: vals["effective_date"], Brand: vals["brand.name"]}
	if d.PublishedAt != nil {
		p.PublishedAt = *d.PublishedAt
	}
	return p, missing, nil
}

// textLang is the language a text is shown in: the one asked for when the
// brand speaks it and the text has it, else the brand's default.
func (s *Service) textLang(t Text, lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if slices.Contains(s.d.Brand.Locales, lang) && t[lang] != "" {
		return lang
	}
	return s.d.Brand.DefaultLocale
}

// values are the placeholders' values for a version in a language.
func (s *Service) values(ctx context.Context, details map[string]string, lang string, d *Document) (map[string]string, error) {
	id, err := s.identity(ctx)
	if err != nil {
		return nil, err
	}
	vals := map[string]string{}
	for k, v := range details {
		vals[k] = v
	}
	vals["brand.name"], vals["support_email"], vals["support_phone"] = id.Name, id.SupportEmail, id.SupportPhone
	vals["emergency_phone"], vals["website"] = id.EmergencyPhone, s.d.Brand.Domains["web"]
	fallback(vals, "legal_entity", id.LegalEntity)
	fallback(vals, "regulator", s.regulatorName())
	fallback(vals, "minimum_age", defaultMinimumAge)
	vals["country.name"] = s.countryName(lang)
	when := s.now()
	if d != nil && d.PublishedAt != nil {
		when = *d.PublishedAt
	}
	vals["effective_date"] = longDate(when.In(s.location()), lang)
	if d != nil {
		vals["version"] = strconv.Itoa(d.Version)
	}
	return vals, nil
}

func fallback(vals map[string]string, key, v string) {
	if strings.TrimSpace(vals[key]) == "" {
		vals[key] = v
	}
}

// identity is the live brand identity, or the brand file's.
func (s *Service) identity(ctx context.Context) (Identity, error) {
	if s.d.Identity != nil {
		return s.d.Identity(ctx)
	}
	b := s.d.Brand
	return Identity{Name: b.DisplayName, LegalEntity: b.LegalEntity, SupportEmail: b.SupportEmail, SupportPhone: b.SupportPhone,
		EmergencyPhone: b.EmergencyPhone}, nil
}

// regulatorName is the home country's regulator code ("LATRA"), or "" when it has none.
func (s *Service) regulatorName() string {
	c := s.d.Brand.CountryByCode(s.d.Brand.Country)
	if c == nil || c.Regulator == "" || c.Regulator == regulatorNone {
		return ""
	}
	return strings.ToUpper(c.Regulator)
}

func (s *Service) countryName(lang string) string {
	c := s.d.Brand.CountryByCode(s.d.Brand.Country)
	if c == nil {
		return s.d.Brand.Country
	}
	if v := pick(c.Name, lang, s.d.Brand.DefaultLocale); v != "" {
		return v
	}
	return c.Code
}

func (s *Service) location() *time.Location {
	if loc, err := time.LoadLocation(s.d.Brand.Timezone); err == nil {
		return loc
	}
	return time.UTC
}

// fillPlain fills placeholders in a plain-text line (a title): no Markdown escaping.
func fillPlain(src string, vals map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(src, func(m string) string {
		if v := strings.TrimSpace(vals[placeholderRe.FindStringSubmatch(m)[1]]); v != "" {
			return v
		}
		return m
	})
}
