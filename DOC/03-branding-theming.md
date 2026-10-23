# 03 · Branding and theming — configurable colours, logos and icons

> Design for making every colour, logo and icon of the Platform configurable from the admin dashboard: pick a curated palette, import one from Color Hunt by URL, or enter custom colours; upload logos and icons; preview; publish to the rider app, driver app, admin, receipts and messages without a code change. Extends D0 (brand is data) in `01-platform-redesign.md`. Decision record: `adr/0006-runtime-theming.md`.

## 1. What is configurable, and when it takes effect

| Item | Where it is used | Changes at runtime (no app release) | Needs an app build (EAS) |
|---|---|---|---|
| Colour palette (light + dark) | apps, admin, receipts/PDF, emails, map markers | ✔ | — |
| Logo (full, mark, on-dark variant) | app headers, splash overlay, receipts, emails, admin | ✔ | — |
| Typography (from an allow-list) and shape (radius) | apps, admin | ✔ | — |
| In-app illustrations, empty states, service tile icons | apps | ✔ | — |
| App name under the icon, launcher icon, native splash, store listing | OS home screen, stores | — | ✔ (generated from the same theme) |

Apps fetch the published theme at start and on foreground; a theme version bump re-renders immediately. Launcher icon and native splash are generated from the same theme into the build assets, so a rebrand is one publish plus one scheduled build.

## 2. Theme model (per brand, versioned)

```
brand_themes
  id, brand_id, version, status DRAFT|PUBLISHED|ARCHIVED, name
  source        PRESET:<preset_key> | IMPORTED:<colorhunt_url> | CUSTOM
  palette       { primary, secondary, accent, background, surface, text, muted, border,
                  success, warning, danger, info }                      -- hex, light mode
  dark          same keys (nullable → derived automatically)
  derived       tonal scales 50…900 for primary/secondary/accent, on-colours (text on primary etc.), map style
  typography    { heading_font, body_font }   from allow-list: Figtree, Plus Jakarta Sans, Inter, Outfit, Poppins, Noto Sans
  shape         { radius_sm, radius_md, radius_lg }
  assets        { logo_full, logo_mark, logo_on_dark, app_icon_1024, splash_image, favicon,
                  marker_driver, marker_pickup, marker_dropoff }        -- object-storage keys, versioned URLs
  wcag_report   contrast results per role pair
  change_reason, created_by, published_at, published_by

theme_presets   key, name, colours[4], source_url, attribution, tags[], suggested_roles
```
Only one PUBLISHED theme per brand. Publishing archives the previous one; rollback republishes it. Every change is audited with a reason (same `X-Change-Reason` rule as money config).

## 3. From a 4-colour palette to a full design system

Color Hunt palettes are four colours. The server derives everything else so a non-designer gets a coherent, accessible result:

1. **Sort by lightness** (OKLCH L). Lightest → `background`; second lightest → `surface`; the most chromatic of the remaining two → `accent`; the other → `primary`. If the lightest colour is too dark for a background (L < 0.85), `background` becomes near-white and the colour becomes `surface`.
2. **Text**: near-black (`#111318`) unless the darkest palette colour reaches 7:1 against `background`, in which case it is used for headings. `muted` and `border` are tints of `text`.
3. **On-colours**: for every filled surface (`primary`, `accent`, `secondary`), text is white or `#111318`, whichever gives the higher contrast; ratio must be ≥ 4.5:1 or publish is blocked.
4. **Tonal scales** 50…900 per brand colour in OKLCH with constant hue, used for hover/pressed/disabled states and charts.
5. **Semantic colours** default to fixed accessible values (`success #1B9C5A`, `warning #E0A100`, `danger #D64545`, `info = primary.500`) and may be overridden; status colours are never reused as chart colours (rule carried from the admin design system).
6. **Dark mode**: derived by inverting lightness roles (`background` ← darkest tone of `primary.900` mixed with neutral, `surface` one step lighter) while keeping `accent` hue; overridable per key.
7. **Map**: light or dark base style chosen from `background` lightness; markers tinted `primary` (drivers), `accent` (pickup), `danger` (drop-off).

The admin can override the automatic role assignment by dragging colours between roles; the derivation re-runs live.

## 4. Admin experience (Branding page, roles Super Admin and Brand Manager)

1. **Identity**: brand name, legal entity, support contacts, domains, store ids (feeds `brands`).
2. **Colours**: gallery of presets (section 6) → one click applies; **"Import from Color Hunt"** accepts a `colorhunt.co/palette/<24 hex chars>` URL and parses the four colours from the URL itself (no scraping); **Custom** gives a picker per role with hex input, eyedropper and an automatic contrast badge; light/dark toggle.
3. **Logos and icons**: upload SVG (preferred) or PNG for full logo, mark and on-dark variant; server rasterises sizes, generates the 1024 app icon, Android adaptive icon (foreground = mark, background = `primary`), iOS icon set, favicon, splash; preview on light and dark.
4. **Typography and shape**: pick from the allow-list; radius presets.
5. **Preview**: live rendering of rider home + service picker, driver offer card + phase screen, receipt PDF, admin sidebar; a QR code opens the draft on a staff device (`theme_preview=<draft_id>`).
6. **Publish**: contrast report must be green; reason required; shows a diff from the current version; one-click rollback. If launcher assets changed, the page says "requires app release" and offers **Export build assets**.

## 5. Delivery

- `GET /v1/app/config` (ETag) returns `{brand, theme:{version, palette, dark, derived, typography, shape, assets}, features, min_app_version}`. Apps cache it (MMKV), apply through a `ThemeProvider` that maps roles to design tokens, and re-apply when `version` changes. The generated TypeScript client types the payload.
- Admin sets CSS variables from the same payload; Next.js keeps no brand constants.
- Receipts (PDF), emails and push use `assets.logo_*` and `palette.primary` from the published theme at render time.
- Fonts are bundled in the apps for the whole allow-list; the theme only selects. Arbitrary font upload is out of scope (unreliable at runtime).
- **Build assets**: `platform brand export --brand <code>` writes `brands/<code>.json` + icon/splash files consumed by Expo `app.config.ts`; CI builds per brand profile.
- Preview tokens are signed, short-lived, staff-only.

## 6. Curated presets (verified on colorhunt.co, 2026-10-14)

Each row links to the Color Hunt page whose title carries the exact hexes. Suggested roles follow the derivation in section 3 and can be changed in the admin.

| Key | Name | Colours | Suggested roles (primary / secondary / accent / background) | Fits |
|---|---|---|---|---|
| `songa-classic` | Songa current identity (not Color Hunt) | `#001F54` `#00B384` `#F4F6FA` `#111318` | 001F54 / 0A3D91 / 00B384 / F4F6FA | continuity for the first brand |
| `deep-navy` | [Deep Navy](https://colorhunt.co/palette/0b244719376d576cbca5d7e8) | `#0B2447` `#19376D` `#576CBC` `#A5D7E8` | 0B2447 / 19376D / 576CBC / white (A5D7E8 as surface tint) | **recommended platform default**: closest to Songa navy, strong contrast |
| `classic-navy` | [Classic Navy](https://colorhunt.co/palette/112d4e3f72afdbe2eff9f7f7) | `#112D4E` `#3F72AF` `#DBE2EF` `#F9F7F7` | 112D4E / 3F72AF / 3F72AF / F9F7F7 | admin console, conservative |
| `royal-gold` | [Royal Blue & Gold](https://colorhunt.co/palette/191d881450a3337ccfffc436) | `#191D88` `#1450A3` `#337CCF` `#FFC436` | 1450A3 / 191D88 / FFC436 / white | taxi brand, high-visibility CTA |
| `blue-mustard` | [Blue & Mustard](https://colorhunt.co/palette/0c356a0174beffc436fff0ce) | `#0C356A` `#0174BE` `#FFC436` `#FFF0CE` | 0C356A / 0174BE / FFC436 / FFF0CE | warm, friendly rider app |
| `ocean-teal` | [Ocean Teal](https://colorhunt.co/palette/0719520b666a35a29f97feed) | `#071952` `#0B666A` `#35A29F` `#97FEED` | 071952 / 0B666A / 35A29F / white | water services, fresh |
| `harbor-blue` | [Harbor Blue](https://colorhunt.co/palette/164863427d9d9bbec8ddf2fd) | `#164863` `#427D9D` `#9BBEC8` `#DDF2FD` | 164863 / 427D9D / 427D9D / DDF2FD | calm, light-first |
| `steel-blue` | [Steel Blue](https://colorhunt.co/palette/27374d526d829db2bfdde6ed) | `#27374D` `#526D82` `#9DB2BF` `#DDE6ED` | 27374D / 526D82 / 526D82 / DDE6ED | operations dashboard |
| `marine-coral` | [Marine & Coral](https://colorhunt.co/palette/3d5a8098c1d9e0fbfcee6c4d) | `#3D5A80` `#98C1D9` `#E0FBFC` `#EE6C4D` | 3D5A80 / 98C1D9 / EE6C4D / E0FBFC | recovery/towing accent |
| `slate-teal` | [Slate & Teal](https://colorhunt.co/palette/222831393e4600adb5eeeeee) | `#222831` `#393E46` `#00ADB5` `#EEEEEE` | 222831 / 393E46 / 00ADB5 / EEEEEE | dark-first driver app |
| `dark-teal` | [Dark Teal](https://colorhunt.co/palette/2c33332e4f4f0e8388cbe4de) | `#2C3333` `#2E4F4F` `#0E8388` `#CBE4DE` | 2E4F4F / 2C3333 / 0E8388 / CBE4DE | dark mode, low glare at night |
| `night-green` | [Night Green](https://colorhunt.co/palette/0f0f0f232d3f005b41008170) | `#0F0F0F` `#232D3F` `#005B41` `#008170` | 005B41 / 232D3F / 008170 / 0F0F0F (dark) | pure dark theme |
| `deep-indigo` | [Deep Indigo](https://colorhunt.co/palette/070f2b1b1a55535c919290c3) | `#070F2B` `#1B1A55` `#535C91` `#9290C3` | 1B1A55 / 070F2B / 9290C3 / white | premium, night |
| `midnight-coral` | [Midnight & Coral](https://colorhunt.co/palette/1a1a2e16213e0f3460e94560) | `#1A1A2E` `#16213E` `#0F3460` `#E94560` | 0F3460 / 16213E / E94560 / white | bold accent, dark surfaces |
| `indigo-apricot` | [Indigo & Apricot](https://colorhunt.co/palette/2d32504247697077a1f6b17a) | `#2D3250` `#424769` `#7077A1` `#F6B17A` | 2D3250 / 424769 / F6B17A / white | friendly, warm CTA |
| `navy-sunset` | [Navy & Sunset](https://colorhunt.co/palette/2d4059ea5455f07b3fffd460) | `#2D4059` `#EA5455` `#F07B3F` `#FFD460` | 2D4059 / — / F07B3F / white; EA5455 → danger, FFD460 → warning | palette with built-in semantic colours |
| `teal-sunrise` | [Teal & Sunrise](https://colorhunt.co/palette/1d5b79468b97ef6262f3aa60) | `#1D5B79` `#468B97` `#EF6262` `#F3AA60` | 1D5B79 / 468B97 / F3AA60 / white; EF6262 → danger | balanced, energetic |
| `safety-orange` | [Safety Orange](https://colorhunt.co/palette/ff6000ffa559ffe6c7454545) | `#FF6000` `#FFA559` `#FFE6C7` `#454545` | 454545 / FFA559 / FF6000 / FFE6C7 | **hauling driver app**: high-vis, truck-yard feel |
| `forest-sand` | [Forest & Sand](https://colorhunt.co/palette/3a4d394f6f52739072ece3ce) | `#3A4D39` `#4F6F52` `#739072` `#ECE3CE` | 3A4D39 / 4F6F52 / 739072 / ECE3CE | eco, water and waste services |
| `racing-green` | [Racing Green & Gold](https://colorhunt.co/palette/004225f5f5dcffb000ffcf9d) | `#004225` `#F5F5DC` `#FFB000` `#FFCF9D` | 004225 / — / FFB000 / F5F5DC | premium green, gold CTA |

Recommendation for the first brand: keep `songa-classic` as the published theme for continuity, with `deep-navy` as the platform default for new brands and `safety-orange` offered as the driver-app accent variant. Preset gallery is data (`theme_presets`), so more palettes are rows, not releases; the Color Hunt import handles anything not in the list.

## 7. Rules that protect quality

- Publish is blocked when body text on `background` < 4.5:1, button text on `primary`/`accent` < 4.5:1, or `primary` vs `accent` are indistinguishable (ΔE < 10).
- Status colours never double as chart colours; charts use the derived sequential scale of `primary` plus four categorical colours derived at fixed hue offsets.
- Logos must have a transparent background and a minimum 512 px mark; SVG is sanitised server-side.
- Every theme version is immutable once published; apps pin the version they rendered with in bug reports.
- Attribution: preset rows store `source_url`; the admin shows "Palette from Color Hunt" on imported/preset themes.

## 8. Phasing

Phase 0 ships the model, `GET /v1/app/config`, the first preset and the ThemeProvider in both apps and the admin (needed anyway for brand-as-data). Phase 2 ships the full Branding page (gallery, import, custom, assets, preview, publish/rollback) and the build-asset export. Dark-mode derivation and the icon generator land with the Phase 2 admin work.
