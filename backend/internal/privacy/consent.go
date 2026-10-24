package privacy

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// latest is an app's newest consent version (nil when the app has none: no gate).
func (s *Service) latest(ctx context.Context, app string) (*Notice, error) {
	all, err := s.notices(ctx)
	if err != nil {
		return nil, err
	}
	list := all[app]
	if len(list) == 0 {
		return nil, nil
	}
	return &list[len(list)-1], nil
}

// Document is an app's current notice in a language (the default language when
// missing), with the documents it covers. ok is false when the app has none.
func (s *Service) Document(ctx context.Context, app, locale string) (*ConsentDocument, bool, error) {
	n, err := s.latest(ctx, app)
	if err != nil || n == nil {
		return nil, false, err
	}
	pick := func(m map[string]string) string {
		if v := m[locale]; v != "" {
			return v
		}
		return m[s.d.Brand.DefaultLocale]
	}
	doc := &ConsentDocument{App: app, Version: n.Version, Title: pick(n.Title), Summary: pick(n.Summary), URL: pick(n.URL),
		PublishedAt: n.PublishedAt, Links: []ConsentLink{}}
	for _, l := range n.Links {
		doc.Links = append(doc.Links, ConsentLink{Kind: l.Kind, Title: pick(l.Title), URL: pick(l.URL)})
	}
	return doc, true, nil
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
	n, err := s.latest(ctx, app)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return &ConsentStatus{App: app}, nil
	}
	acc, err := s.accepted(ctx, userID, app)
	if err != nil {
		return nil, err
	}
	return &ConsentStatus{App: app, CurrentVersion: n.Version, AcceptedVersion: acc, Required: acc == nil || *acc < n.Version}, nil
}

// Required reports whether a person must accept the app's current notice first.
func (s *Service) Required(ctx context.Context, userID, app string) (bool, error) {
	if app == "" {
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
	n, err := s.latest(ctx, in.App)
	if err != nil {
		return 0, err
	}
	if n == nil {
		return 0, ErrInvalid
	}
	if in.Version != n.Version {
		return n.Version, ErrConsentStale
	}
	install := ""
	if u, err := uuid.Parse(in.InstallID); err == nil {
		install = u.String()
	}
	if err := s.store.insertConsent(ctx, userID, in, install); err != nil {
		return n.Version, err
	}
	if s.d.Redis != nil {
		s.d.Redis.Set(ctx, s.cacheKey(userID, in.App), n.Version, ConsentCacheTTL)
	}
	return n.Version, nil
}
