package branding

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
)

// cacheCheckTTL bounds how long another instance serves an old theme after a publish.
const cacheCheckTTL = 5 * time.Second

// Live is what app config serves: the identity and the published theme.
type Live struct {
	Identity Identity
	Theme    *theme.Theme
}

// Service is the branding use cases.
type Service struct {
	store    store
	files    Files
	fallback *theme.Theme // the brand file's preset, until a theme is published
	presets  *theme.Presets
	live     *cache.Versioned[Live]
}

// New wires the service; fallback is served when no theme is published yet.
func New(pool *pgxpool.Pool, brandID string, files Files, presets *theme.Presets, fallback *theme.Theme) *Service {
	s := &Service{store: store{pool: pool, brandID: brandID}, files: files, fallback: fallback, presets: presets}
	s.live = cache.New("branding", s.store, s.loadLive, cacheCheckTTL)
	return s
}

// Published returns the live identity and theme (cached; one version check per few seconds).
func (s *Service) Published(ctx context.Context) (Live, error) {
	l, _, err := s.live.Get(ctx)
	return l, err
}

func (s *Service) loadLive(ctx context.Context) (Live, error) {
	id, err := s.store.identity(ctx)
	if err != nil {
		return Live{}, err
	}
	t, err := s.store.published(ctx)
	if err == ErrThemeNotFound {
		return Live{Identity: id, Theme: s.fallback}, nil
	}
	if err != nil {
		return Live{}, err
	}
	return Live{Identity: id, Theme: toTheme(t)}, nil
}

// toTheme is a stored version as the apps receive it (asset keys become public links).
func toTheme(t *ThemeVersion) *theme.Theme {
	out := &theme.Theme{Version: t.Version, Palette: t.Palette, Dark: t.Dark, Typography: t.Typography, Shape: t.Shape, Assets: assetURLs(t.Assets)}
	if key, ok := strings.CutPrefix(t.Source, "PRESET:"); ok {
		out.Preset = key
	}
	return out
}

// Identity returns the brand's identity for the editor.
func (s *Service) Identity(ctx context.Context) (Identity, error) { return s.store.identity(ctx) }

// SetIdentity changes the brand's identity (audited); the brand file stops overwriting it.
func (s *Service) SetIdentity(ctx context.Context, staffID string, in IdentityInput) (Identity, error) {
	if err := validateIdentity(&in); err != nil {
		return Identity{}, err
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.setIdentity(ctx, tx, in, staffID); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, s.store.brandID, staffID, "branding.identity", "brand:"+s.store.brandID, in.Reason, in); err != nil {
			return err
		}
		return s.store.bump(ctx, tx)
	})
	if err != nil {
		return Identity{}, err
	}
	return s.store.identity(ctx)
}

func validateIdentity(in *IdentityInput) error {
	in.Name, in.Reason = strings.TrimSpace(in.Name), strings.TrimSpace(in.Reason)
	var fields []map[string]string
	if in.Name == "" || len([]rune(in.Name)) > NameMaxLen {
		fields = append(fields, map[string]string{"path": "name", "code": "LENGTH"})
	}
	if e := strings.TrimSpace(in.SupportEmail); e != "" {
		if _, err := mail.ParseAddress(e); err != nil {
			fields = append(fields, map[string]string{"path": "support_email", "code": "FORMAT"})
		}
	}
	if len(fields) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	if in.Reason == "" {
		return ErrReasonRequired
	}
	return nil
}

// Versions lists every theme version, newest first.
func (s *Service) Versions(ctx context.Context) ([]ThemeVersion, error) {
	list, err := s.store.themes(ctx)
	for i := range list {
		list[i].AssetURLs = assetURLs(list[i].Assets)
	}
	return list, err
}

// Get returns one version.
func (s *Service) Get(ctx context.Context, id string) (*ThemeVersion, error) {
	t, err := s.store.theme(ctx, s.store.pool, id, false)
	if err == nil {
		t.AssetURLs = assetURLs(t.Assets)
	}
	return t, err
}

// Presets is the curated palette gallery.
func (s *Service) Presets() []theme.Preset { return s.presets.List() }
