package privacy

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// current is an app's newest notice (false when the brand lists none: no gate).
func (s *Service) current(app string) (int, bool) {
	docs := s.docs[app]
	if len(docs) == 0 {
		return 0, false
	}
	return docs[len(docs)-1].Version, true
}

// Document is an app's current notice in a language (the default language when missing).
func (s *Service) Document(app, locale string) (*ConsentDocument, bool) {
	docs := s.docs[app]
	if len(docs) == 0 {
		return nil, false
	}
	d := docs[len(docs)-1]
	pick := func(m map[string]string) string {
		if v := m[locale]; v != "" {
			return v
		}
		return m[s.d.Brand.DefaultLocale]
	}
	return &ConsentDocument{App: app, Version: d.Version, Title: pick(d.Title), Summary: pick(d.Summary), URL: pick(d.URL), PublishedAt: d.PublishedAt}, true
}

// SyncDocuments records the brand file's notices (the versions people accept refer to them).
func (s *Service) SyncDocuments(ctx context.Context) error {
	for _, docs := range s.docs {
		for _, d := range docs {
			if err := s.store.upsertDocument(ctx, d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) cacheKey(userID, app string) string {
	return fmt.Sprintf("consent:%s:%s:%s", s.d.Brand.Code, userID, app)
}

// accepted is the newest version a person accepted for an app (Redis, then Postgres).
func (s *Service) accepted(ctx context.Context, userID, app string) (*int, error) {
	if s.d.Redis != nil {
		if v, err := s.d.Redis.Get(ctx, s.cacheKey(userID, app)).Result(); err == nil {
			if n, err := strconv.Atoi(v); err == nil {
				return &n, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			return nil, err
		}
	}
	v, err := s.store.acceptedVersion(ctx, userID, app)
	if err == nil && v != nil && s.d.Redis != nil {
		s.d.Redis.Set(ctx, s.cacheKey(userID, app), *v, ConsentCacheTTL)
	}
	return v, err
}

// Status is where a person stands with an app's notice.
func (s *Service) Status(ctx context.Context, userID, app string) (*ConsentStatus, error) {
	cur, ok := s.current(app)
	if !ok {
		return &ConsentStatus{App: app}, nil
	}
	acc, err := s.accepted(ctx, userID, app)
	if err != nil {
		return nil, err
	}
	return &ConsentStatus{App: app, CurrentVersion: cur, AcceptedVersion: acc, Required: acc == nil || *acc < cur}, nil
}

// Required reports whether a person must accept the app's current notice first.
func (s *Service) Required(ctx context.Context, userID, app string) (bool, error) {
	if _, ok := s.current(app); !ok || app == "" {
		return false, nil
	}
	st, err := s.Status(ctx, userID, app)
	if err != nil {
		return false, err
	}
	return st.Required, nil
}

// Accept records that a person accepted the current version (append-only).
func (s *Service) Accept(ctx context.Context, userID string, in AcceptInput) (int, error) {
	cur, ok := s.current(in.App)
	if !ok {
		return 0, ErrInvalid
	}
	if in.Version != cur {
		return cur, ErrConsentStale
	}
	install := ""
	if u, err := uuid.Parse(in.InstallID); err == nil {
		install = u.String()
	}
	if err := s.store.insertConsent(ctx, userID, in, install); err != nil {
		return cur, err
	}
	if s.d.Redis != nil {
		s.d.Redis.Set(ctx, s.cacheKey(userID, in.App), cur, ConsentCacheTTL)
	}
	return cur, nil
}
