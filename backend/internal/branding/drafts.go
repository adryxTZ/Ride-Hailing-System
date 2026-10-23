package branding

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
)

// PreviewRoles derives the light and dark palettes and their report from roles (nothing saved).
func PreviewRoles(r Roles) Preview {
	light := theme.Derive(r.Light)
	dark := theme.DeriveDark(light, r.Dark)
	return Preview{Palette: light, Dark: dark, Report: theme.Evaluate(light, &dark)}
}

// ImportColorHunt reads a Color Hunt link into four colours and suggested roles.
func ImportColorHunt(link string) ([]string, map[string]string, error) {
	colours, err := theme.FromColorHunt(link)
	if err != nil {
		return nil, nil, ErrNotColorHuntURL
	}
	return colours, theme.AssignRoles(colours), nil
}

// CreateDraft saves a new draft version.
func (s *Service) CreateDraft(ctx context.Context, staffID string, in DraftInput) (*ThemeVersion, error) {
	return s.saveDraft(ctx, staffID, "", in)
}

// UpdateDraft replaces a draft (published and archived versions never change).
func (s *Service) UpdateDraft(ctx context.Context, staffID, id string, in DraftInput) (*ThemeVersion, error) {
	return s.saveDraft(ctx, staffID, id, in)
}

func (s *Service) saveDraft(ctx context.Context, staffID, id string, in DraftInput) (*ThemeVersion, error) {
	if err := validateDraft(&in); err != nil {
		return nil, err
	}
	if err := s.checkAssets(ctx, in.Assets); err != nil {
		return nil, err
	}
	p := PreviewRoles(in.Roles)
	t := &ThemeVersion{Name: in.Name, Source: in.Source, Roles: in.Roles, Palette: p.Palette, Dark: &p.Dark,
		Typography: in.Typography, Shape: in.Shape, Assets: in.Assets, Report: p.Report}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if id != "" {
			cur, err := s.store.theme(ctx, tx, id, true)
			if err != nil {
				return err
			}
			if cur.Status != StatusDraft {
				return ErrNotDraft
			}
		}
		var err error
		if id, err = s.store.writeDraft(ctx, tx, id, staffID, t); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "branding.theme.draft", "theme:"+id, "", map[string]any{"source": in.Source})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// DeleteDraft removes a draft that was never published.
func (s *Service) DeleteDraft(ctx context.Context, staffID, id string) error {
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.deleteDraft(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotDraft
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "branding.theme.delete", "theme:"+id, "", nil)
	})
}

// validateDraft checks names, roles, fonts, radii and asset slots against the allow-lists.
func validateDraft(in *DraftInput) error {
	in.Name = strings.TrimSpace(in.Name)
	var bad []map[string]string
	flag := func(path, code string) { bad = append(bad, map[string]string{"path": path, "code": code}) }
	if in.Name == "" || len([]rune(in.Name)) > NameMaxLen {
		flag("name", "LENGTH")
	}
	if !validSource(in.Source) {
		flag("source", "FORMAT")
	}
	checkRoles("roles.light", in.Roles.Light, flag)
	checkRoles("roles.dark", in.Roles.Dark, flag)
	if in.Roles.Light["primary"] == "" {
		flag("roles.light.primary", "REQUIRED")
	}
	for k, v := range in.Typography {
		if !slices.Contains(FontKeys, k) || !slices.Contains(Fonts, v) {
			flag("typography."+k, "NOT_ALLOWED")
		}
	}
	for k, v := range in.Shape {
		if limit, ok := ShapeMax[k]; !ok || v < 0 || v > limit {
			flag("shape."+k, "OUT_OF_RANGE")
		}
	}
	for k, v := range in.Assets {
		if !slices.Contains(AssetSlots, k) || !strings.HasPrefix(v, assetKeyPrefix) {
			flag("assets."+k, "NOT_ALLOWED")
		}
	}
	if len(bad) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": bad})
	}
	return nil
}

func checkRoles(path string, roles map[string]string, flag func(path, code string)) {
	for k, v := range roles {
		if !slices.Contains(RoleKeys, k) {
			flag(path+"."+k, "NOT_ALLOWED")
		} else if !theme.ValidHex(v) {
			flag(path+"."+k, "FORMAT")
		}
	}
}

func validSource(src string) bool {
	return src == "CUSTOM" || strings.HasPrefix(src, "PRESET:") || strings.HasPrefix(src, "IMPORTED:")
}
