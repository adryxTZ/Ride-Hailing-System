# frontend/ — web surfaces

| Folder | What | Stack | Status |
|---|---|---|---|
| `admin/` | Operations dashboard (drivers, riders, trips, finance, pricing, safety, audit, staff) | Next.js, generated API client from `backend/api/openapi.yaml`, brand/theme from `GET /v1/app/config` | built (Phase D, 2026-10-20): overview, live, trips, drivers, riders, pricing, finance, safety, audit, staff |
| `website/` | Public marketing site, white-label (brand data drives name, colours, copy) | React + Vite (reuse Songa website structure, brand-neutral) | placeholder |

Mobile apps live in `apps/`; the Go backend in `backend/`. Shared TypeScript packages used by both `frontend/` and `apps/` live in `packages/`.
