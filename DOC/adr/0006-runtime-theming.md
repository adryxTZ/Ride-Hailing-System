# ADR 0006 · Runtime theming: colours, logos and icons are versioned brand data chosen in the admin

Date: 2026-10-14 · Status: Accepted

## Context
Kalen: both app and system colours must be configurable by the operator from the admin dashboard, from colour palettes (Color Hunt) or custom values, together with logo and icons. Songa hard-codes navy/green, the logo and the name in two Flutter apps, the admin and SMS templates.

## Decision
- A versioned `brand_themes` record per brand (palette light/dark, derived scales and on-colours, typography from an allow-list, shape, logo/icon/splash/marker assets, WCAG report), DRAFT → PUBLISHED with audit reason and rollback; `theme_presets` as a curated, data-driven gallery.
- Admin Branding page: preset gallery, import from a Color Hunt URL (hexes parsed from the URL), custom per-role colours with live contrast checks, asset uploads with generated icon sets, live preview, publish.
- Apps and admin consume the published theme from `GET /v1/app/config` through a ThemeProvider; receipts, emails and push render with it. Launcher icon, app name and native splash are generated from the same theme into build assets and need an app build.
- Publishing is blocked when contrast fails WCAG AA for text on background and buttons.

## Alternatives
- Build-time theming only (brand JSON compiled into the apps): every colour tweak becomes a store release.
- Free-form CSS/token upload: no quality guard, breaks accessibility and charts.

## Consequences
Phase 0 must ship the theme model and ThemeProvider; the full Branding page lands in Phase 2 (≈ 3 engineer-weeks). Design tokens, not literal colours, everywhere in apps and admin; a lint rule forbids raw hex in components.
