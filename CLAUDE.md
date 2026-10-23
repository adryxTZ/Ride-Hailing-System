# Project: multi-service mobility & logistics platform (working folder "RIDER & HAULER")

This folder is a **greenfield redesign**. The existing Songa system at
`/Users/kalen/PycharmProject/songa` is the *reference* only: read it for parity
checks, never copy its design. Songa's apps belong to a separate developer and
must not be edited.

## Conventions
- All documents and working notes live in `DOC/`. Start with `DOC/README.md`.
- The product is **generic and white-label**: the system/brand name is a variable
  (`PLATFORM_NAME`, brand rows), never a literal in code, schemas or docs. Docs say
  "the Platform". Songa and Rider & Hauler are brands, not identifiers.
- Decisions (2026-10-14): **Go** backend, single binary with roles; **React Native
  (Expo)** rider + driver apps in one monorepo; Next.js admin kept and repointed;
  Postgres + PostGIS + Redis; River for jobs; **clean start** (reference data only
  carried over from Songa); dispatch follows Uber's principles (H3, ETA-based
  batched assignment, sequential offers with broadcast fallback).
- **Taxi + Boda packages (Kalen, 2026-10-20, ADR 0009):** the Platform is a ride-hailing
  app like Bolt/Uber: `taxi` with ride types as vehicle classes (Boda, Bajaji, Standard,
  XL) plus `parcel` ("Package", Boda only, receiver required), like Bolt Send. No other
  delivery, hauling, recovery, water or food services: do not add service pickers,
  cargo, loading/unloading or per-service forms back.
- Every Songa flaw listed in `DOC/01-platform-redesign.md` §D12 is a review check:
  do not reintroduce any of them.
- Architecture decisions are recorded as ADRs in `DOC/adr/` (0001–0009).
- Never add AI attribution trailers to commits in this repo.

## Repository layout (Kalen, 2026-10-14)
- Three self-contained areas, each with its own dependencies and tooling; nothing
  is installed at the repo root: `backend/` (Go service `nexa`, owns the API contract
  at `backend/api/openapi.yaml`), `apps/` (mobile: rider, driver + `apps/packages/`,
  own pnpm workspace and node_modules), `frontend/` (web: admin dashboard, website +
  `frontend/packages/`, own pnpm workspace). Both TS workspaces generate their API
  client from the backend's contract (file path in the monorepo, or
  `GET /v1/openapi.yaml` from a running backend). **No build-time path references
  across areas** (ADR 0007): each area has its own Dockerfile/CI/lockfile and can be
  hosted or split out alone. Brand runtime data: `backend/brands/`; app build
  identity: `apps/brands/`. `infra/` (root) composes for hosting; `DOC/` docs.

## Where things are
- `DOC/00-songa-system-scan.md` — what Songa is and how it works (reference).
- `DOC/01-platform-redesign.md` — target architecture, stack, data model, API, apps, infra, phases, risks.
- `DOC/02-dispatch-engine.md` — booking + matching engine design.
- `DOC/03-branding-theming.md` — configurable colours/logos/icons (admin-selectable palettes, Color Hunt import, custom), theme delivery to apps.
- `DOC/04-pricing-modules.md` — pricing layers and the pluggable engine registry (how to add a module).
- `DOC/reference/songa-vision/` — Kalen's product-vision documents (proposal PDFs, Songa docs).

## Engineering standards (Kalen, 2026-10-14: "comment, separate concerns so code is easy to maintain")
- **One concern per file, one responsibility per function.** Transport (handlers/DTOs),
  domain logic (services), persistence (stores/queries) and models live in separate
  files and packages; handlers only parse input, call a service port and write a response.
- **Every package has a `doc.go`** stating its purpose and file layout; every exported
  identifier has a doc comment; non-obvious decisions get a short "why" comment.
- Go: `internal/<module>/` holds `service.go` (use cases), `model.go` (domain types),
  `store.go` (sqlc-backed persistence), `ports.go` (interfaces this module needs),
  `handlers_*.go` only inside `kernel/httpx`. No business logic in `cmd/`.
- Apps: `screens/` (views), `components/` (presentational), `hooks/` (behaviour),
  `services/` (API/realtime adapters), `models/` (types) — never mix in one file.
- Small functions (aim ≤ 40 lines), explicit names, no magic numbers (named constants).
- Tests sit next to the code (`*_test.go`, `*.test.ts`); behaviour tests over mocks.
- `gofmt`, `go vet`, `golangci-lint`, Biome/ESLint and the brand-name grep must pass
  before any commit.

## Performance standards (Kalen, 2026-10-14: "caching to increase speed, reduce infrastructure load like DB calls")
- **Read-mostly data is cached, not queried.** Catalogue, brand/theme, tariffs, config:
  in-process cache (seconds to minutes) keyed by a Redis `cfg:version` counter that
  admin writes bump; Redis as the shared cache tier; Postgres only on miss or write.
- **HTTP caching on every public GET**: ETag + `Cache-Control`; clients send
  `If-None-Match` and get 304s (already done for `/app/config`).
- **Hot paths never touch Postgres**: presence, matching, surge, rate limits and the
  GPS stream live in Redis; trips write to Postgres once per transition, not per ping.
- **No N+1**: one query per use case with joins or `ANY($1)`; batch inserts for
  telemetry; pool bounded (`MaxConns` 20) and indexes for every filter used.
- **Measure before and after**: p95 latency and queries-per-request are part of the
  definition of done for every endpoint; `EXPLAIN ANALYZE` for anything over 10 ms.

## Design standards (Kalen, 2026-10-14)
- Consistency first: every screen uses `@platform/ui` primitives (Screen, Header, Section, ListRow, Button, TextField, Icon, Avatar, Banner, PillTabBar) and theme tokens only. Reference boards: `DOC/designs/` (general screens, concept, taxi story).
- **Motion is part of the design**: staggered entrances for grids/lists (Reanimated `FadeInDown.delay(i*60)`), `ScalePressable` on every card/tile, layout transitions on selection (tab pill, option grids), **eased timing, never springs or bounces** (Kalen, 2026-10-14: "remove the bouncing animation"; ~240–280 ms ease-out, presses ~110 ms). No screen ships without its entrance and press states.
- Cards: 18px radius, soft shadow (opacity 0.05, radius 12, elevation 2), surface tint from theme; icons are MaterialCommunityIcons glyph names stored in catalogue data.
- Brand palette is configurable (current default preset `nexa-violet`, primary #5003C0); no literal colours in components.
