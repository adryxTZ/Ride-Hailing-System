package legal

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// Publish makes the document's draft the live version, with a reason (audited).
// The previous live version is archived. For a text people accept, the app's
// consent version moves on in the same transaction, so riders or drivers are
// asked to accept again; every API instance reloads within seconds.
func (s *Service) Publish(ctx context.Context, staffID, app, kind, reason string) (*Document, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > ReasonMaxLen {
		return nil, ErrReasonRequired
	}
	if !Offered(app, kind) {
		return nil, ErrNotFound
	}
	var id string
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		d, err := s.lockedDraft(ctx, tx, app, kind)
		if err != nil {
			return err
		}
		id = d.ID
		if err := s.store.publish(ctx, tx, d, staffID, reason); err != nil {
			return err
		}
		consentVersion, err := s.afterPublishTx(ctx, tx, app, kind)
		if err != nil {
			return err
		}
		data := map[string]any{"app": app, "kind": kind, "version": d.Version, "consent_version": consentVersion}
		if err := audit.Record(ctx, tx, s.store.brandID, staffID, "legal.publish", "legal:"+d.ID, reason, data); err != nil {
			return err
		}
		return s.store.bump(ctx, tx)
	})
	if err != nil {
		return nil, err
	}
	return s.byID(ctx, app, kind, id)
}

// lockedDraft is the draft to publish, checked complete in every brand language.
func (s *Service) lockedDraft(ctx context.Context, tx pgx.Tx, app, kind string) (*Document, error) {
	if err := s.store.lock(ctx, tx, docKey(app, kind)); err != nil {
		return nil, err
	}
	d, err := s.store.draft(ctx, tx, app, kind)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNoDraft
	}
	if err != nil {
		return nil, err
	}
	if bad := s.readyToPublish(d); len(bad) > 0 {
		return nil, ErrInvalid.WithDetails(map[string]any{"fields": bad})
	}
	return d, nil
}

// afterPublishTx moves the app's consent version on when the kind is accepted
// by people (0 when it is not, or no consent record is wired).
func (s *Service) afterPublishTx(ctx context.Context, tx pgx.Tx, app, kind string) (int, error) {
	k, _ := Info(kind)
	if !k.Consent || s.d.Consent == nil {
		return 0, nil
	}
	v, err := s.consentVersion(ctx, tx, app, nil)
	if err != nil {
		return 0, err
	}
	return s.d.Consent.NewVersionTx(ctx, tx, v)
}

// consentVersion describes what the app's people accept now: its live texts
// that need consent, their links and the Privacy Notice's summary.
func (s *Service) consentVersion(ctx context.Context, tx pgx.Tx, app string, title Text) (ConsentVersion, error) {
	live, err := s.store.live(ctx, tx)
	if err != nil {
		return ConsentVersion{}, err
	}
	v := ConsentVersion{App: app, Title: title, Documents: map[string]int{}}
	for _, k := range Kinds {
		d := findDoc(live, app, k.Kind)
		if d == nil || !k.Consent {
			continue
		}
		link := ConsentLink{Kind: k.Kind, Title: d.Title, URL: s.pageURLs(app, k.Kind)}
		v.Links = append(v.Links, link)
		v.Documents[k.Kind] = d.Version
		if k.Kind == KindPrivacyNotice {
			v.URL, v.Summary = link.URL, d.Summary
		}
	}
	return v, nil
}

// pageURLs is a document's public page per brand language.
func (s *Service) pageURLs(app, kind string) Text {
	out := Text{}
	for _, lang := range s.d.Brand.Locales {
		out[lang] = PagePath(app, kind, lang)
	}
	return out
}

func findDoc(docs []Document, app, kind string) *Document {
	for i := range docs {
		if docs[i].App == app && docs[i].Kind == kind {
			return &docs[i]
		}
	}
	return nil
}
