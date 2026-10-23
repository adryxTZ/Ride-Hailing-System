# infra — running and hosting the Platform

`infra/` composes the three areas (ADR 0007) for running and hosting. It holds no build
logic for them: each image builds from its own folder (`backend/`, `frontend/`, `apps/`).

| Path | What it is |
|---|---|
| `up.sh`, `compose.yml` | One-command local run of everything in Docker |
| `compose.local.yml` | Postgres + Redis only, for `make dev-up` while running the API from source |
| `deploy/` | Server stack for UAT and production: compose + overlays, Caddy, monitoring, backups, ops scripts, maps |
| `loadtest/` | k6 load test (see `loadtest/README.md`) |

---

## 1. One-command local run

```
./infra/up.sh            # Postgres, Redis, migrate + seed, API :8080, admin :3000, mobile bundlers :8081-8084
./infra/up.sh core       # same without the mobile bundlers
./infra/up.sh staff add 0712345678 --name "Ops" --roles "Super Admin" --email you@example.com --password '<password>'
./infra/up.sh down       # stop, keep data        ./infra/up.sh reset   # stop and delete data
```

`up.sh` builds the images, waits for every healthcheck (`--wait`), then checks
`GET /v1/health` and the admin `/login` page. Ports and the project are variables, so a
second copy runs beside the developer stack:

```
COMPOSE_PROJECT_NAME=nexa-test API_PORT=18080 ADMIN_PORT=13000 PG_PORT=15433 REDIS_PORT=16380 \
BACKEND_IMAGE=nexa-test-backend:local ADMIN_IMAGE=nexa-test-admin:local ./infra/up.sh core
docker compose -p nexa-test -f infra/compose.yml down -v     # tear down
```

Local sign-in uses the development OTP settings in `compose.yml` (fixed code for the test
numbers, codes echoed). These defaults exist only in the local file; servers set none.

---

## 2. Hosting (UAT and production)

**Topology** (D8, D11): one host per environment, never shared (D12 #24). Each runs
`caddy` (TLS) → `api` (all roles in one container) + `admin`, plus `redis` and `alloy`
(monitoring). Postgres is **managed and external** (PostGIS, point-in-time recovery on).
About 1 GB RAM in use on a 2 GB droplet at pilot scale.

| File in `deploy/` | Purpose |
|---|---|
| `compose.yml` | Services, healthchecks, resource limits, log rotation (json-file, 5 × 20 MB), volumes; profiles `ops` (one-off jobs) and `maps` |
| `compose.uat.yml` / `compose.production.yml` | Per-environment sizing; production also has the `split` profile (separate worker container) |
| `caddy/Caddyfile` | TLS (automatic Let's Encrypt), security headers, websocket proxying, internal metrics on :9180 |
| `monitoring/` | `config.alloy`, `postgres-queries.yml` (River queue depth), `rules/alerts.yml` |
| `backup/` | Backup toolbox image (pg_dump 16, age, rclone) with `backup.sh` / `restore.sh` |
| `ops/deploy.sh` | The deploy: pull → migrate → seed → `up -d --wait` → smoke |
| `ops/smoke.sh`, `ops/verify-ledger.sh`, `ops/crontab.example` | Smoke test, daily ledger check, cron lines |
| `maps/prepare.sh` | OSM extracts → OSRM graph + Nominatim input |
| `env/*.example` | Templates for the files that live only on the host |

### First-time host setup

1. Ubuntu 24.04 LTS, Docker Engine + compose plugin, a `deploy` user in the `docker` group,
   firewall open for 22/80/443 only. DNS: `api.<domain>` and `admin.<domain>` (or
   `uat-api.` / `uat-admin.`) point at the host.
2. `sudo mkdir -p /opt/platform /var/lib/platform/metrics && sudo chown -R deploy /opt/platform /var/lib/platform`
3. In `/opt/platform`, from `infra/deploy/env/`: `compose.env.example` → `.env`,
   `api.env.example` → `api.env`, `backup.env.example` → `backup.env`,
   `monitoring.env.example` → `monitoring.env`; fill them in, `chmod 600 *.env`. Provider key
   files (Selcom payout, regulator) go in `/opt/platform/secrets/` (mode 600).
4. The managed database: create the app user, enable `postgis`, create a read-only
   `monitor` role with `GRANT SELECT ON river_job TO monitor`.
5. Run the first deploy from GitHub (below). Then create the first staff account:
   `docker compose -f compose.yml -f compose.<env>.yml --profile ops run --rm seed staff add <phone> --name "<name>" --roles "Super Admin" --email <email> --password '<temporary>'`
6. Install the cron lines from `ops/crontab.example` (backup 02:15, ledger verify 03:05 UTC).

### Deploys

`ops/deploy.sh <api-image> <admin-image>`, run by CI over SSH (or by hand):

1. pulls the images (the tags are written to `.env`, the host's release record, only after step 5 passes);
2. **migrates** in a one-off container (`nexa migrate`) before new code serves;
3. seeds reference data (idempotent);
4. `docker compose up -d --wait`: containers must pass healthchecks (`/app/probe` → `/v1/ready`);
5. smoke test through Caddy: `/v1/health`, `/v1/ready`, `/v1/app/config`, admin `/login`.

There is no watchtower and no building on servers: images are immutable, tagged by commit.
**Rollback** = run the `deploy` workflow by hand with the previous tags. Migrations are
forward-only, so each release must keep working with the previous release's code
(add columns first, remove them a release later).

**Rehearsal**: the stack was exercised on a laptop on 2026-10-23 with `SKIP_PULL=1`
(local images), `SMOKE_INSECURE=1`, `PUBLIC_HTTPS_PORT=18443` and `api.localhost` /
`admin.localhost` (Caddy issues local certificates for `*.localhost`).

---

## 3. CI/CD (`.github/workflows/`)

| Workflow | When | What |
|---|---|---|
| `backend.yml` | PR touching `backend/`; called by release | gofmt, go vet, golangci-lint, brand-name grep (`make brand-check`), build, migrate + seed, `go test -race` with Postgres/PostGIS + Redis service containers and `NEXA_TEST_DATABASE_URL`, OpenAPI lint, **generated-client staleness** for both TS workspaces |
| `frontend.yml` | PR touching `frontend/` or the contract; called by release | `tsc`, Biome, `next build` |
| `apps.yml` | PR touching `apps/` or the contract; called by release | `tsc`, Biome, Jest |
| `release.yml` | push to `main`, tags `v*` | runs the three checks → builds and pushes `nexa:<sha>` and `admin:<sha>-<env>` to GHCR → **UAT** deploy on main, **production** deploy on tags |
| `deploy.yml` | called by release, or by hand (rollback) | rsync `infra/deploy/` to the host (never env files), registry login, `ops/deploy.sh`, then smoke from outside with the expected build version |
| `apps-eas.yml` | by hand | EAS builds per brand, app, profile and platform (section 4) |

Production deploys need approval: create the GitHub Environment **`production`** with
required reviewers and restrict it to tags `v*`. One deploy per environment runs at a time
(concurrency group), so migrations never race.

### Required secrets and variables

| Name | Kind | Where | Value |
|---|---|---|---|
| `DEPLOY_HOST` | secret | Environments `uat`, `production` | Host name or IP |
| `DEPLOY_USER` | secret | both | `deploy` |
| `DEPLOY_SSH_KEY` | secret | both | Private key of a deploy-only key pair (public key in the host's `authorized_keys`) |
| `DEPLOY_KNOWN_HOSTS` | secret | both | Output of `ssh-keyscan <host>` (host key pinned) |
| `GHCR_READ_TOKEN` | secret | both | Fine-grained PAT, `read:packages` only |
| `GHCR_READ_USER` | variable | both | The PAT owner's user name |
| `API_DOMAIN`, `ADMIN_DOMAIN` | variable | both | e.g. `uat-api.<domain>`, `uat-admin.<domain>` |
| `DEPLOY_PATH` | variable | both (optional) | Default `/opt/platform` |
| `UAT_PUBLIC_API_URL` | variable | repository | `https://uat-api.<domain>/v1` (baked into the UAT admin image) |
| `PRODUCTION_PUBLIC_API_URL` | variable | repository | `https://api.<domain>/v1` |
| `EXPO_TOKEN` | secret | repository | Expo robot token (EAS builds) |

`GITHUB_TOKEN` pushes the images; nothing else is needed for GHCR.

---

## 4. Store builds (EAS)

- **Identity per brand**: `apps/brands/<code>.json` (display name, slugs, bundle ids /
  packages, deep-link scheme, API hosts) is read by `apps/brands/expo-brand.js` from both
  `app.config.ts` files. For EAS add `"eas_owner": "<expo account>"` and in `rider` / `driver`
  `"eas_project_id": "<uuid>"` (from `eas init` in each app folder, once per brand).
- **Assets per brand**: publish the theme (icon 1024×1024 PNG, splash) in the admin
  Branding page, then from `apps/`: `node tools/brand-assets.mjs --brand <code> --env prod`.
  It writes `apps/brands/<code>/icon.png`, `splash.png`, other logos and `theme.json`;
  commit them. Without them builds use Expo's default icon. The splash needs
  `expo-splash-screen`, which is declared in both apps (run `pnpm install` once).
- **Profiles** (`apps/rider/eas.json`, `apps/driver/eas.json`): `development` (dev client,
  internal, channel `development`; add `expo-dev-client` to the app first, after which
  `expo start` needs `--go` for Expo Go), `preview` (internal APK / ad-hoc, `APP_ENV=uat`,
  channel `uat`), `production` (store, `APP_ENV=prod`, channel `prod`, build numbers
  auto-incremented remotely).
- **Brand on the builder**: local environment variables are not sent to EAS, so set
  `BRAND` on each brand's EAS projects: `eas env:create --name BRAND --value <code>
  --environment production --environment preview --environment development --visibility plaintext`.
- **Build**: `cd apps/rider && BRAND=<code> eas build --profile preview --platform android`,
  or the `apps-eas` workflow.
- **Signing (no keys in the repository)**: run `eas credentials` per app once. Android:
  let EAS generate the upload keystore and enrol in Play App Signing. iOS: sign in with the
  Apple Developer account and EAS creates the distribution certificate and provisioning
  profiles. Push: upload the FCM V1 service-account JSON (Android) and an APNs key (iOS)
  in `eas credentials`. Back up the Android upload keystore from EAS to the password
  manager. Store submission (`eas submit`) needs a Play service-account key and an App
  Store Connect API key, both held by EAS.

---

## 5. Monitoring and alerts

Alloy (`deploy/monitoring/config.alloy`) sends to Grafana Cloud (free tier is enough at pilot
scale). Every series carries `env=uat|production`.

| Source | Series used by alerts |
|---|---|
| Caddy `:9180/metrics` (per host; API requests get `service="api"`) | `caddy_http_request_duration_seconds_*` → API p95, 5xx rate, no traffic |
| Postgres exporter + `postgres-queries.yml` | `pg_river_queue_jobs`, `pg_river_oldest_available_seconds`, `pg_river_discarded_recent_jobs` |
| Host (node exporter) + textfile collector | disk and memory; `platform_ledger_verify_*` (from `ops/verify-ledger.sh`) and `platform_backup_*` (from the backup job) |
| Docker logs → Loki | all containers; the API's JSON request log (`path`, `status`, `ms`) for drill-down |

Rules (`deploy/monitoring/rules/alerts.yml`, checked with `promtool`): API p95 > 150 ms for
10 min (warning) and > 500 ms for 5 min (critical), 5xx > 1 %, no API traffic, River backlog
> 500 or oldest job > 5 min, discarded jobs, ledger verify failed or older than 26 h, disk
< 15 % / < 5 %, memory < 10 %, backup failed or older than 26 h. Load them with
`mimirtool rules load deploy/monitoring/rules/alerts.yml` (Grafana Cloud) and route
critical alerts to the on-call phone.

---

## 6. Backups and the restore drill

- **First line**: managed Postgres point-in-time recovery (provider console).
- **Off-provider copy**: the nightly `backup` job (`ops/crontab.example`) writes
  `db-<stamp>.dump.age` (pg_dump custom format) and `files-<stamp>.tar.gz.age` (uploads + JWT
  keys) to `store:<bucket>/<prefix>/<env>/daily/`, Sunday copies to `weekly/`. Daily dumps
  are kept `BACKUP_RETENTION_DAYS` (14), weekly ones `BACKUP_WEEKLY_RETENTION_DAYS` (90).
- **Encryption**: `age` public key on the host; the private key (`age-keygen` output) lives
  only in the password manager, so a compromised host cannot read old backups.

**Restore drill (quarterly, and before launch)**, on the UAT host or a laptop:

1. Create an empty database (managed: a new database or a fork; local:
   `createdb restore_drill` with PostGIS available).
2. Put the private key in a file readable only by you, e.g. `~/drill/identity.txt`.
3. Restore the latest dump:
   ```
   docker compose -f compose.yml -f compose.uat.yml --profile ops run --rm \
     -v ~/drill:/key:ro -e AGE_IDENTITY_FILE=/key/identity.txt \
     backup restore daily/db-<stamp>.dump.age 'postgres://<user>:<pw>@<host>/restore_drill?sslmode=require'
   ```
4. Verify: `NEXA_DATABASE_URL=<restore_drill url> nexa verify-ledger` passes; row counts of
   `users`, `trips`, `ledger_entries` match the source at the dump time; start an API
   against it and sign in to the dashboard.
5. Record date, dump age, restore time and result in `DOC/release/`; delete the drill database
   and the key file.

Rehearsed on 2026-10-23 against a throwaway local stack: backup (database + files) encrypted
and uploaded through an rclone `local` remote, restored into a new database (all 67 tables;
users and staff roles matched the source), `verify-ledger` passed.

**Disaster recovery**: restore into a **new** managed database (never over the live one),
verify as above, set `NEXA_DATABASE_URL` in `api.env`, run `ops/deploy.sh` with the current tags.

---

## 7. Maps: self-hosted routing and places (optional)

The backend picks routing from the brand file (`providers.routing: "osrm"`) and reads:

| Env (api.env) | With the `maps` profile | Without |
|---|---|---|
| `NEXA_OSRM_URL` | `http://osrm:5000` | public OSRM demo server (development only) |
| `NEXA_NOMINATIM_URL` | `http://nominatim:8080` | public Nominatim (development only; strict usage policy) |

```
MAPS_DATA_DIR=/var/lib/platform/maps ./maps/prepare.sh             # Tanzania + Kenya
COUNTRIES="tanzania kenya uganda" ./maps/prepare.sh                # other Geofabrik africa/* extracts
docker compose -f compose.yml -f compose.<env>.yml --profile maps up -d osrm nominatim
```

`prepare.sh` downloads the Geofabrik extracts (MD5-checked), merges them with osmium and
builds the OSRM MLD graph (car profile). Nominatim imports the merged file on first start
(an hour or more; ~4 GB RAM), then serves. Re-run monthly. For TZ + KE plan an extra
**4 GB RAM** (a separate 4–8 GB maps host is sensible for production) and about 10 GB of
disk. Google keys stay server-side only if chosen instead (D11).

---

## 8. Known gaps (outside infra)

- The backend's object store is the local disk (`NEXA_UPLOAD_DIR`), kept in the `uploads`
  volume and covered by the backup. An S3 adapter is needed before running more than one API host.
- `nexa migrate` takes no advisory lock; the deploy runs migrations one at a time
  (CI concurrency group + `flock`), which is enough for a single host.
- There is no daily ledger-verify River job yet; `ops/verify-ledger.sh` from cron covers it.
