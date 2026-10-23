# ADR 0007 · backend, apps and frontend are independently buildable and deployable

Date: 2026-10-14 · Status: Accepted

## Context
Kalen: "structure it in such a way that it can be hosted apart or split and not cause any issue", with node_modules, APIs and tooling living in their respective folders, and `infra/` staying at the root because it is responsible for hosting.

## Decision
- Three self-contained areas: `backend/` (Go, owns the API contract at `backend/api/openapi.yaml` and the runtime brand data in `backend/brands/`), `apps/` (mobile pnpm workspace with `apps/packages/` and `apps/brands/` build identity), `frontend/` (web pnpm workspace with `frontend/packages/`). Nothing is installed at the repo root.
- **No path references across areas at build time.** The only shared thing is the API contract, consumed as an artifact: each TS workspace commits its generated client, regenerated from a file path when the monorepo is present or from `GET /v1/openapi.yaml` on any running backend (`OPENAPI_SPEC=<url> pnpm gen:api`).
- Each area has its own Dockerfile/build with its folder as the build context, its own CI workflow filtered on its path, its own lockfile and lint config.
- `infra/` at the root composes the areas for single-host hosting; it never contains build logic.
- Brand data is deliberately split: runtime data (colours, legal entity, providers) is served by the backend; build identity (bundle ids, slugs, API hosts) sits with the apps. Duplicating the brand code and display name there is intentional.

## Consequences
Splitting into three repositories is `git subtree split` per folder plus moving its workflow file; nothing else changes. The price is that a contract change needs `pnpm gen:api` in two places, enforced by CI's "generated client is committed and current" check.
