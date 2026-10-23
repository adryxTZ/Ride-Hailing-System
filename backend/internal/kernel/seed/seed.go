// Package seed loads reference data from files into the database: brands from
// brands/<code>.json and theme presets from seed/theme_presets.json. Seeding is
// idempotent (upsert by natural key) so it can run on every deploy.
package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
)

// Brand upserts one brand row and, if it has no published theme yet, publishes
// version 1 from the brand's preset so /app/config has something to serve.
func Brand(ctx context.Context, pool *pgxpool.Pool, b *brand.Brand, presets *theme.Presets) error {
	domains, _ := json.Marshal(b.Domains)
	providers, _ := json.Marshal(b.Providers)
	var brandID string
	err := pool.QueryRow(ctx, `
		INSERT INTO brands (code, display_name, legal_entity, country, currency, timezone, default_locale, locales, domains, support_phone, support_email, providers, emergency_phone)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (code) DO UPDATE SET
		  -- Identity is the brand file's only until staff edit it on the Branding page.
		  display_name=CASE WHEN brands.identity_edited_at IS NULL THEN EXCLUDED.display_name ELSE brands.display_name END,
		  legal_entity=CASE WHEN brands.identity_edited_at IS NULL THEN EXCLUDED.legal_entity ELSE brands.legal_entity END,
		  support_phone=CASE WHEN brands.identity_edited_at IS NULL THEN EXCLUDED.support_phone ELSE brands.support_phone END,
		  support_email=CASE WHEN brands.identity_edited_at IS NULL THEN EXCLUDED.support_email ELSE brands.support_email END,
		  emergency_phone=CASE WHEN brands.identity_edited_at IS NULL THEN EXCLUDED.emergency_phone ELSE brands.emergency_phone END,
		  country=EXCLUDED.country, currency=EXCLUDED.currency, timezone=EXCLUDED.timezone, default_locale=EXCLUDED.default_locale,
		  locales=EXCLUDED.locales, domains=EXCLUDED.domains, providers=EXCLUDED.providers, updated_at=now()
		RETURNING id`,
		b.Code, b.DisplayName, nilIfEmpty(b.LegalEntity), b.Country, b.Currency, b.Timezone, b.DefaultLocale,
		b.Locales, domains, nilIfEmpty(b.SupportPhone), nilIfEmpty(b.SupportEmail), providers, nilIfEmpty(b.EmergencyPhone),
	).Scan(&brandID)
	if err != nil {
		return fmt.Errorf("upsert brand %s: %w", b.Code, err)
	}

	th, ok := presets.Resolve(b.ThemePreset)
	if !ok {
		return fmt.Errorf("brand %s: unknown theme preset %q", b.Code, b.ThemePreset)
	}
	palette, _ := json.Marshal(th.Palette)
	typography, _ := json.Marshal(th.Typography)
	preset, _ := presets.Get(b.ThemePreset)
	roles, _ := json.Marshal(map[string]any{"light": preset.Roles, "dark": map[string]string{}})
	_, err = pool.Exec(ctx, `
		INSERT INTO brand_themes (brand_id, version, status, name, source, palette, typography, roles, published_at)
		SELECT $1, 1, 'PUBLISHED', $2, $3, $4, $5, $6, now()
		WHERE NOT EXISTS (SELECT 1 FROM brand_themes WHERE brand_id = $1)`,
		brandID, th.Preset, "PRESET:"+th.Preset, palette, typography, roles)
	if err != nil {
		return fmt.Errorf("publish initial theme for %s: %w", b.Code, err)
	}
	// Until staff publish a theme on the Branding page, the brand file owns the
	// look: the automatic version 1 follows the file's preset.
	_, err = pool.Exec(ctx, `
		UPDATE brand_themes SET name=$2, source=$3, palette=$4, typography=$5, roles=$6
		WHERE brand_id=$1 AND version=1 AND published_by IS NULL AND source <> $3
		  AND NOT EXISTS (SELECT 1 FROM brand_themes WHERE brand_id=$1 AND version > 1)`,
		brandID, th.Preset, "PRESET:"+th.Preset, palette, typography, roles)
	if err != nil {
		return fmt.Errorf("refresh initial theme for %s: %w", b.Code, err)
	}
	_, err = pool.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='branding'`)
	return err
}

// Presets upserts the curated palette gallery.
func Presets(ctx context.Context, pool *pgxpool.Pool, presets *theme.Presets) error {
	for _, p := range presets.List() {
		roles, _ := json.Marshal(p.Roles)
		if _, err := pool.Exec(ctx, `
			INSERT INTO theme_presets (key, name, colours, roles, source_url, tags)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (key) DO UPDATE SET name=EXCLUDED.name, colours=EXCLUDED.colours,
			  roles=EXCLUDED.roles, source_url=EXCLUDED.source_url, tags=EXCLUDED.tags`,
			p.Key, p.Name, p.Colours, roles, p.SourceURL, p.Tags); err != nil {
			return fmt.Errorf("upsert preset %s: %w", p.Key, err)
		}
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
