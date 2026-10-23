# DOC — multi-service mobility & logistics platform (working folder "RIDER & HAULER")

All documents and working notes for the greenfield redesign live here. The Platform is **generic and white-label**: the system name is a variable; Songa and Rider & Hauler are brands, not identifiers. Songa (`/Users/kalen/PycharmProject/songa`) is the reference system being replaced.

## Reading order

| # | Document | What it is |
|---|---|---|
| 0 | `00-songa-system-scan.md` | How Songa works today: architecture, APIs, trip lifecycle, pricing, payments, apps, admin, infra, and its design flaws. Reference only. |
| 1 | `01-platform-redesign.md` | The target: decisions, product vision, white-label foundation (D0), Go modular monolith (D1–D2), service abstraction (D3), data model (D4), dispatch summary (D5), API (D6), React Native (D7), infra and delivery (D8), phases (D9), risks (D10), review refinements (D11), **Songa flaws removed** (D12), open decisions. |
| 2 | `02-dispatch-engine.md` | Booking flow and the driver-suggestion engine: presence, DispatchSaga, cost function v0→v2, batched assignment, offers, broadcast fallback, knobs, data, metrics, failure modes, tests. |
| 3 | `03-branding-theming.md` | Configurable branding: versioned theme model, admin Branding page (presets, Color Hunt import, custom colours, logos/icons), delivery to apps/admin/receipts, WCAG guards, 19 verified Color Hunt presets with recommendations. |
| 4 | `04-pricing-modules.md` | Pricing layers (manual > fixed route > zone > metered), the shipped modules, how an operator picks one, how a developer adds one. |
| — | `adr/` | Architecture decision records 0001–0008 (Go; monolith + River + Redis; service abstraction; dispatch model; brand as data; runtime theming; independently deployable areas; pluggable pricing modules). |
| — | `reference/songa-vision/` | Kalen's product-vision documents: the three Songa proposals (Platform, Tracking, Analysis) and the Songa engineering docs. Inputs, kept verbatim. |

## Variables

`PLATFORM_NAME`/`brand.name`, `BRAND`, `COUNTRY`, `CURRENCY`, `REGULATOR`, `GATEWAY`, `SMS_PROVIDER`, `ROUTING`, codename `platform` — defined in `01-platform-redesign.md`.

## Status (2026-10-14)

- Scan, redesign, dispatch design, branding/theming design and ADRs written. **Phase 0 scaffolding started 2026-10-14** — see `notes/2026-10-14-phase0-progress.md`.
- Open decisions are listed at the end of `01-platform-redesign.md` §4.
- Conventions for Claude Code sessions: `../CLAUDE.md`. Open sessions from the project folder so this project gets its own memory.
- `06-taxi-roadmap.md` — taxi-only plan (2026-10-20): what is done, the phases that remain, in order.
