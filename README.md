# The Platform — ride-hailing and Boda package delivery

A white-label taxi app in the style of Bolt and Uber: ride types Boda, Bajaji, Standard and XL, plus Boda package delivery. The system name, colours, logos and prices are **brand data**, never code. Internal codename: `nexa`; the default brand is **Nexa**, `songa` is a second example brand.

| Area | Path | Stack |
|---|---|---|
| Backend | `backend/` | Go, one binary (`nexa`) with roles api/realtime/worker/scheduler; Postgres + PostGIS, Redis, River jobs |
| API contract | `backend/api/openapi.yaml` | Owned by the backend; `apps/` and `frontend/` generate their typed clients from it |
| Mobile apps | `apps/rider`, `apps/driver` | React Native (Expo SDK 54), one pnpm workspace with shared `apps/packages/` |
| Admin dashboard | `frontend/admin` | Next.js, generated API client, brand theme from `GET /v1/app/config` |
| Brands | `backend/brands/<code>.json` (runtime), `apps/brands/<code>.json` (build identity) | ADR 0005 and 0007 |
| Infra | `infra/` | Local compose (Postgres, Redis); each area also builds and deploys alone |
| Docs | `DOC/` | Design documents, ADRs, roadmap (`DOC/06-taxi-roadmap.md`), factory specs (`DOC/specs/`) |

## Prerequisites
- Docker Desktop (Postgres and Redis run in containers)
- Go 1.23 or newer, `golangci-lint`
- Node 20 or newer, `pnpm` (`corepack enable`)
- For phones: Expo Go on the phone, `adb` for Android over USB, an Expo account (`npx expo login`) so iPhones can open the dev server
- Optional: Playwright browsers for the browser tests (`npx playwright install chromium` inside `apps/`)

## Run everything locally

### 1. Database and Redis
```bash
docker compose -f infra/compose.local.yml up -d postgres redis
```
Postgres listens on `localhost:5433` (user, password and database all `nexa`), Redis on `localhost:6380`.

### 2. Backend API (port 8080)
```bash
cd backend
./tools/dev-serve.sh
```
The script builds the binary, applies migrations, loads the seed data (catalogue, prices, fee policy, brands, theme presets) and starts `nexa serve` on `0.0.0.0:8080` with:
- test sign-in for the apps: code `0000` for the numbers `0626512332` (rider) and `0715123702` (driver); any other number gets its code echoed in the response (development only). Dashboard staff sign in with email and password.
- the **fake payment gateway**, which settles mobile-money prompts after 2 s (phone numbers ending `0001`, `0002`, `0003`, `0004` script a failure, a rejection, a timeout and an outage)
- logs in `backend/tmp/nexa.log`

Useful backend commands (from `backend/`):
```bash
make test            # all Go tests; integration tests need NEXA_TEST_DATABASE_URL=postgres://nexa:nexa@localhost:5433/nexa?sslmode=disable
make lint            # golangci-lint
make brand-check     # no brand name under internal/ or cmd/
./bin/nexa staff add 07XXXXXXXX --roles "Super Admin" --email you@example.com --password secret   # dashboard account
./bin/nexa driver provision --phone 07XXXXXXXX                    # approve a test driver with a vehicle
```
Environment variables are read from the shell; see `backend/internal/kernel/config/config.go` for the full list (database, Redis, OTP, payment gateway, Selcom credentials, webhook base URL).

### 3. Admin dashboard (port 3000)
```bash
cd frontend
pnpm install
cp admin/.env.example admin/.env.local     # NEXT_PUBLIC_API_URL=http://localhost:8080/v1
pnpm --filter admin dev
```
Open http://localhost:3000 and sign in with a staff email and password. Create the first account with `./bin/nexa staff add 07XXXXXXXX --roles "Super Admin" --email you@example.com --password ...` (from `backend/`); more staff are added on the dashboard's Staff page. Checks: `pnpm --filter admin typecheck`, `pnpm lint`. After a contract change: `pnpm gen:api`.

### 4. Mobile apps (rider and driver)
```bash
cd apps
pnpm install
./tools/dev-metro.sh
```
This starts four bundlers: rider on `:8081` and driver on `:8082` for phones, rider web on `:8083` and driver web on `:8084` for browsers (open them to try the apps without a phone). Logs are in `apps/tools/logs/`.

On phones:
- **Android over USB**: `./tools/usb-phones.sh` sets up `adb reverse` so the phone reaches Metro and the API at `127.0.0.1`, then opens Expo Go. Re-run it if ADB restarts.
- **iPhone over Wi-Fi**: the Mac and the phone must be on the same network and the Expo CLI must be signed in to the same Expo account as Expo Go (`npx expo login` once). Scan the QR code printed by the bundler.
- Sign in with the test numbers above. The driver must be approved before going online (`nexa driver provision`, or review the registration in the dashboard under Drivers).

Checks: `pnpm typecheck && pnpm lint`, `pnpm test`. After a contract change: `pnpm gen:api`. Brand build: `BRAND=<code> APP_ENV=local|uat|prod`.

### 5. End-to-end tests (from `apps/`, with the API on :8080)
```bash
node tools/e2e-dispatch.mjs        # book, match, drive, complete, chat, rate
node tools/e2e-cancellation.mjs    # cancellation fees and dues
node tools/e2e-safety.mjs          # share link and SOS
node tools/e2e-payin.mjs           # debt holds and mobile-money pay-ins
node tools/e2e-fares.mjs           # rider fares by mobile money, retries, cash, receipts (PDF links)
node tools/e2e-payouts.mjs         # driver cash-outs: approve, pay, reject, cancel
node tools/e2e-admin.mjs           # every dashboard page (admin on :3000)
node tools/e2e-web-hold.mjs        # driver web: hold panel and Pay now (driver web on :8084)
node tools/e2e-web-*.mjs           # rider web flows (rider web on :8083)
node tools/shot-admin.mjs /finance # screenshots into tools/shots/ (THEME=dark for dark mode)
```

## Production notes
- Set `NEXA_PAYMENT_GATEWAY=selcom` with `NEXA_SELCOM_BASE_URL`, `NEXA_SELCOM_API_KEY`, `NEXA_SELCOM_API_SECRET`, `NEXA_SELCOM_VENDOR` and `NEXA_PAYMENT_WEBHOOK_BASE_URL`; the server refuses to start with Selcom and missing credentials, and never falls back to the fake gateway silently.
- Driver cash-outs use Selcom's Business API: set `NEXA_SELCOM_PAYOUT_BASE_URL`, `NEXA_SELCOM_PAYOUT_API_KEY` and `NEXA_SELCOM_PAYOUT_KEY_FILE` (the RSA private key registered with Selcom). Without them payouts are off and drivers are told so.
- Set `NEXA_PUBLIC_API_URL` (e.g. https://api.example.com) so receipt PDF links point at the public API.
- Never set `NEXA_OTP_DEV_ECHO` or `NEXA_OTP_TEST_CODE` in production.
- Each of `backend/`, `apps/` and `frontend/` deploys from its own folder (ADR 0007). Regenerate a TS client against a deployed backend with `OPENAPI_SPEC=https://<api>/v1/openapi.yaml pnpm gen:api`.

## Rules
- Nothing under `backend/internal/` may contain a brand name (CI greps for it).
- Every API change goes through `backend/api/openapi.yaml` first; clients are generated, never hand-written.
- Decisions live in `DOC/adr/`; the Songa flaw list in `DOC/01-platform-redesign.md` §D12 is a review checklist.
- Features are built through the software factory: `/factory <feature>` (spec, build, four parallel reviews, approval); agents live in `.claude/agents/`.
