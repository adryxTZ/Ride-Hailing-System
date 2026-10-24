package privacy

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Notice is one consent version of an app as stored: what people accept.
type Notice struct {
	App         string
	Version     int
	Title       map[string]string
	Summary     map[string]string
	URL         map[string]string
	Links       []NoticeLink
	PublishedAt time.Time
}

// NoticeLink is a document a consent version covers, with its page per language.
type NoticeLink struct {
	Kind  string            `json:"kind"`
	Title map[string]string `json:"title"`
	URL   map[string]string `json:"url"`
}

// NewNotice is a consent version the legal module publishes. Empty Title or
// Summary keep the previous version's.
type NewNotice struct {
	App       string
	Title     map[string]string
	Summary   map[string]string
	URL       map[string]string
	Links     []NoticeLink
	Documents map[string]int // legal kind -> version
}

// NoticeSource loads every app's consent versions, oldest first.
type NoticeSource func(ctx context.Context) (map[string][]Notice, error)

// notices is the cached consent versions (reloaded when the "legal" scope moves).
func (s *Service) notices(ctx context.Context) (map[string][]Notice, error) {
	if s.d.Notices != nil {
		return s.d.Notices(ctx)
	}
	n, _, err := s.cache.Get(ctx)
	return n, err
}

// NewVersionTx records a new consent version for an app in the caller's
// transaction (a newly published text); the next gate check asks again.
func (s *Service) NewVersionTx(ctx context.Context, tx pgx.Tx, n NewNotice) (int, error) {
	prev, err := s.store.lockLatest(ctx, tx, n.App)
	if err != nil {
		return 0, err
	}
	version := 1
	if prev != nil {
		version = prev.Version + 1
		n.Title, n.Summary = orElse(n.Title, prev.Title), orElse(n.Summary, prev.Summary)
	}
	if len(n.Title) == 0 && len(n.Links) > 0 {
		n.Title = n.Links[0].Title
	}
	return version, s.store.insertNotice(ctx, tx, version, n)
}

// SeedVersionTx links an app's consent to the starter texts: the latest
// version (from the brand file) gets their links without asking anyone again;
// with no version yet, version 1 is written.
func (s *Service) SeedVersionTx(ctx context.Context, tx pgx.Tx, n NewNotice) (int, error) {
	prev, err := s.store.lockLatest(ctx, tx, n.App)
	if err != nil {
		return 0, err
	}
	if prev == nil {
		return s.NewVersionTx(ctx, tx, n)
	}
	return prev.Version, s.store.linkNotice(ctx, tx, prev.Version, n)
}

// SeedDocuments writes the brand file's notices for an app that has no consent
// version in the database yet (the fallback when no legal texts are seeded).
func (s *Service) SeedDocuments(ctx context.Context) error {
	have, err := s.store.notices(ctx)
	if err != nil {
		return err
	}
	for _, d := range s.d.Brand.Legal.Consent {
		if len(have[d.App]) > 0 {
			continue
		}
		if err := s.store.insertBrandNotice(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

func orElse(v, prev map[string]string) map[string]string {
	if len(v) > 0 {
		return v
	}
	return prev
}
