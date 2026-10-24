package legal

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// field is one entry of a VALIDATION_FAILED answer's details.fields.
type field = map[string]string

// SaveDraft creates the document's next version as a draft, or replaces the
// open draft (one per document; published versions never change).
func (s *Service) SaveDraft(ctx context.Context, staffID, app, kind string, in DraftInput) (*Document, error) {
	if !Offered(app, kind) {
		return nil, ErrNotFound
	}
	if bad := s.checkDraft(&in); len(bad) > 0 {
		return nil, ErrInvalid.WithDetails(map[string]any{"fields": bad})
	}
	var id string
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.lock(ctx, tx, docKey(app, kind)); err != nil {
			return err
		}
		cur, err := s.store.draft(ctx, tx, app, kind)
		switch {
		case errors.Is(err, ErrNotFound):
			id, err = s.store.insertDraft(ctx, tx, app, kind, staffID, in)
		case err == nil:
			id, err = cur.ID, s.store.updateDraft(ctx, tx, cur.ID, in)
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "legal.draft", "legal:"+id, "", map[string]any{"app": app, "kind": kind})
	})
	if err != nil {
		return nil, err
	}
	return s.byID(ctx, app, kind, id)
}

// DeleteDraft discards the document's draft.
func (s *Service) DeleteDraft(ctx context.Context, staffID, app, kind string) error {
	if !Offered(app, kind) {
		return ErrNotFound
	}
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		cur, err := s.store.draft(ctx, tx, app, kind)
		if errors.Is(err, ErrNotFound) {
			return ErrNoDraft
		}
		if err != nil {
			return err
		}
		if err := s.store.deleteDraft(ctx, tx, cur.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "legal.draft.delete", "legal:"+cur.ID, "", map[string]any{"version": cur.Version})
	})
}

// byID returns one version of a document.
func (s *Service) byID(ctx context.Context, app, kind, id string) (*Document, error) {
	list, err := s.store.versions(ctx, app, kind)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, ErrNotFound
}

// checkDraft trims the texts and lists what is wrong: languages the brand does
// not speak, lengths, and placeholders that are not in the catalogue.
func (s *Service) checkDraft(in *DraftInput) []field {
	var bad []field
	for _, part := range []struct {
		name string
		text *Text
		max  int
	}{{"title", &in.Title, TitleMaxLen}, {"summary", &in.Summary, SummaryMaxLen}, {"body", &in.Body, BodyMaxBytes}} {
		if *part.text == nil {
			*part.text = Text{}
		}
		bad = append(bad, s.checkText(part.name, *part.text, part.max)...)
	}
	return bad
}

func (s *Service) checkText(name string, t Text, maxLen int) []field {
	var bad []field
	for lang, v := range t {
		path := name + "." + lang
		v = strings.TrimSpace(v)
		t[lang] = v
		size := len([]rune(v))
		if name == "body" {
			size = len(v)
		}
		switch {
		case !slices.Contains(s.d.Brand.Locales, lang):
			bad = append(bad, field{"path": path, "code": "NOT_ALLOWED"})
		case size > maxLen:
			bad = append(bad, field{"path": path, "code": "LENGTH"})
		}
		if unknown := UnknownPlaceholders(v); len(unknown) > 0 {
			bad = append(bad, field{"path": path, "code": "UNKNOWN_PLACEHOLDER", "detail": strings.Join(unknown, ", ")})
		}
	}
	return bad
}

// readyToPublish lists the languages a draft still lacks a title or body in.
func (s *Service) readyToPublish(d *Document) []field {
	var bad []field
	for _, lang := range s.d.Brand.Locales {
		if d.Title[lang] == "" {
			bad = append(bad, field{"path": "title." + lang, "code": "REQUIRED"})
		}
		if d.Body[lang] == "" {
			bad = append(bad, field{"path": "body." + lang, "code": "REQUIRED"})
		}
	}
	return bad
}
