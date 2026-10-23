# Songa — Server Operations & Deploy Skill

Authoritative runbook for managing the Songa backend **in production on the droplet**. If you are
a fresh chat/engineer asked to "deploy", "check the server", "fix prod", or "manage Songa", read
this first. Every command and fact here was verified against the live server.

> Golden rules (read these before doing anything)
> 1. **The deploy branch is `main` (as of 2026-08-17).** The server compose was switched to the
>    `:latest` tags (backup: `/root/songa-backend-app-v2.yml.bak.pretagswitch`), so pushing to
>    `main` builds `:latest` and watchtower deploys it. The `test` branch is deprecated.
> 2. **The pipeline never runs DB migrations.** If your change has a migration, you MUST apply it
>    by hand using the coordinated sequence below, or prod breaks with "column ... does not exist".
> 3. **Server-side secrets live in `/root/.env`, not in the repo and not in GitHub → the container.**
> 4. Confirm hard-to-reverse actions (money code, migrations, prod restarts) before running them.

---

## 1. Access & topology

- **Server:** `ssh root@161.35.167.245` (public IP `161.35.167.245`). Ubuntu droplet, hostname
  `kalen-main`. Docker + docker compose v2.
- **Compose project:** name `root`, file **`/root/songa-backend-app-v2.yml`** (working_dir `/root`).
  Run compose as: `docker compose -p root -f /root/songa-backend-app-v2.yml <cmd>`.
- **Backend image:** `ghcr.io/kalen-limited/songa_backend:latest` (one image shared by API + workers).
- **Containers** (all on that image except flower/infra):
  - `root-songa_backend-1` — the Django/uvicorn API (port 8080→8000)
  - `root-celery_worker-1` — Celery worker (dispatch, payments, fraud, reconcile tasks)
  - `root-celery_beat-1` — Celery beat scheduler
  - `root-django_kafka_consumer-1` — Kafka consumer
  - `root-celery_flower-1` — Celery monitoring UI (non-critical; often lags images, ignore)
  - Infra (separate/shared): `root-kafka-1`, `redis-server`, `pgbouncer`, plus **watchtower**.
- **Inside the container:** app root `/app`, venv python **`/app/env/bin/python`**, `manage.py` at
  `/app`. The entrypoint runs uvicorn — it does **NOT** run migrations.
- Other unrelated stacks share the host (traefik, portainer, corridor-*, tccia, lsf, etc.). They
  show up as compose "orphan containers" — **never** pass `--remove-orphans`; it would delete them.

## 2. CI/CD — how code reaches the server

There is **CI (build) but no CD (release)**. GitHub Actions only builds & publishes images;
**watchtower** on the droplet is what actually deploys them.

- **Workflow:** `.github/workflows/deploy.yml`, triggers on push to `main` and `test`.
  - Builds & pushes 3 images to GHCR: `songa_backend`, `netty_ws_gateway`, `songa_nginx`.
  - Tag rule: push to `test` → image tag `:test`; **push to `main` → tag `:latest` (DEPLOYED)**.
  - Last build step runs `makemigrations --check --dry-run` inside the freshly built image (with
    dummy env) to fail loudly if a model changed without a migration.
  - A second job `deploy-selcom-config` (test branch only) SSHes the Selcom secrets to the droplet
    (see §7).
- **Watchtower** polls GHCR (~12s) and, when the `:latest` digest changes, recreates every container
  using that image (backend, worker, beat, consumer). It ONLY swaps the image and reuses the
  container's existing env. It knows nothing about GitHub secrets or migrations.
- **Therefore:** the running production code = whatever is on branch **`main`**. (Until
  2026-08-17 it was `test`/`:test`; the compose file now pins `:latest`.)

Transient `502` during a watchtower recreate is normal; it settles once the container is up
(`restarts=0`).

## 3. Standard deploy (NO migrations) — the easy path

If `git diff --name-only <deployed>..HEAD | grep migrations/` is empty:

```bash
# from a clone of songa_backend_revamp_v2
git push origin main
```
Watchtower auto-builds+deploys within a few minutes. Verify:
```bash
ssh root@161.35.167.245 'b=$(docker inspect --format "{{.Image}}" root-songa_backend-1); \
  l=$(docker inspect --format "{{.Id}}" ghcr.io/kalen-limited/songa_backend:test); \
  echo "on_latest=$([ "$b" = "$l" ] && echo YES || echo NO) $(docker ps --filter name=root-songa_backend-1 --format "{{.Status}}")"'
```

## 4. Coordinated deploy WITH migrations — the careful path

The container entrypoint does **not** migrate. If your change adds/edits migrations, applying the
code before the schema = production 500s. Apply migrations FIRST (they're additive-safe against the
old code), then recreate. Sequence that has worked repeatedly:

```bash
CF=/root/songa-backend-app-v2.yml
# 1) stop watchtower so it can't recreate onto the new image before migrations run
ssh root@161.35.167.245 'docker stop watchtower'
# 2) push to test; wait for CI to publish the new :test image
git push origin main:test
# 3) on the droplet: pull the new image WITHOUT recreating, then run migrations from it
ssh root@161.35.167.245 "docker compose -p root -f $CF pull songa_backend"
ssh root@161.35.167.245 "docker compose -p root -f $CF run --rm --no-deps --entrypoint sh \
  songa_backend -c 'cd /app && ./env/bin/python manage.py migrate <app> --noinput'"   # per app, or 'migrate' for all
# 4) recreate the services onto the new image
ssh root@161.35.167.245 "docker compose -p root -f $CF up -d songa_backend celery_worker celery_beat django_kafka_consumer"
# 5) resume watchtower
ssh root@161.35.167.245 'docker start watchtower'
```
Always `showmigrations`/`migrate --plan` first. Postgres DDL is transactional, so a bad migration
rolls back. Watch for `AlterField`/`AddField` on existing money tables and `RunSQL` triggers —
those are the ones that break old code or lock rows (e.g. the ledger-immutability trigger
`payments_ledgerentry_immutable`, which blocks UPDATE/DELETE on `LedgerEntry`).

## 5. Mount-test recipe (test code against prod-like env WITHOUT deploying)

Bundle changed files, scp to `/root/<x>_test/`, run inside the image with the test env:
```bash
scp mychange.py root@161.35.167.245:/tmp/mychange.py
ssh root@161.35.167.245 'docker run --rm --network root_songa-net \
  --env-file /root/surge_test/.env.test \
  -v /tmp/mychange.py:/app/path/to/mychange.py:ro \
  ghcr.io/kalen-limited/songa_backend:test \
  sh -c "cd /app && ./env/bin/python your_script.py"'
```
`/root/surge_test/.env.test` is a **test** DB/env — safe for read checks and dry runs. For
prod-DB actions use `docker exec root-songa_backend-1 ...` instead.

Quick prod DB shell:
```bash
ssh root@161.35.167.245 'docker exec root-songa_backend-1 ./env/bin/python manage.py shell'
```

## 6. Env & secrets model

- The compose services get env via **inline `environment:` blocks** in
  `/root/songa-backend-app-v2.yml` (DB, NextSMS, AWS, Google Maps all literal there). To add a new
  env var you edit those blocks (per service). A backup is at
  `/root/songa-backend-app-v2.yml.bak.preselcom`.
- Selcom vars are wired via `${VAR}` substitution from **`/root/.env`** (mode 600). `/root/.env`
  is read by `docker compose --env-file /root/.env` for `${}` substitution only — it is NOT an
  `env_file:` for the container.
- **Never** print secret values in logs/output. Check presence, not value:
  ```bash
  docker exec root-songa_backend-1 ./env/bin/python -c \
    "import os;print('X:', 'SET' if os.environ.get('X') else 'MISSING')"
  ```

## 7. Selcom payments — the full story

Multi-gateway payout/collection layer (Selcom Checkout: driver USSD debt pay-in + rider card).

- **Config** (`songaServer/settings.py` `PAYMENT_GATEWAYS['SELCOM']`, via `python-decouple`):
  - `SELCOM_CHECKOUT_API_KEY` → falls back to **`SELCOM_API_KEY`**
  - `SELCOM_CHECKOUT_API_SECRET` → falls back to **`API_SECRET`**
  - `SELCOM_VENDOR_ID` (default `70053934`)
  - `SELCOM_CHECKOUT_BASE_URL` (default `https://apigw.selcommobile.com/v1` — already carries `/v1`;
    a bug where code appended a second `/v1` was fixed in commit `558ca8e`)
  - `PUBLIC_BASE_URL` (e.g. `https://songa.kalen.co.tz`) → webhook URL
    `{PUBLIC_BASE_URL}/api/payments/webhooks/selcom/`
- **How creds reach the container:** they are **not** in GitHub→image. They live in `/root/.env`,
  put there by the CI job `deploy-selcom-config` in `deploy.yml` (test branch), which SSHes to the
  droplet using a **dedicated, forced-command-restricted** deploy key:
  - Server script: **`/root/bin/deploy-selcom.sh`** (mode 700). Reads `KEY=VALUE` from **stdin**,
    accepts only an allowlist (`SELCOM_API_KEY API_SECRET SELCOM_VENDOR_ID PUBLIC_BASE_URL`), never
    eval's, upserts `/root/.env` (600), recreates the 3 services, prints SET/MISSING.
  - `~/.ssh/authorized_keys` pins the CI key to `restrict,command="/root/bin/deploy-selcom.sh"` —
    a leaked key can run ONLY that script, never a shell.
  - **Required GitHub repo secrets:** `DROPLET_HOST`, `DROPLET_USER`, `DROPLET_SSH_KEY` (private
    half of the forced-command key), `DROPLET_SSH_HOSTKEY` (pinned host key line), plus
    `SELCOM_API_KEY`, `API_SECRET`, `SELCOM_VENDOR_ID`, `PUBLIC_BASE_URL`.
  - To re-run: push to `test` (or GitHub → Actions → re-run failed job). Adding secrets does NOT
    auto-trigger; you must push or re-run.
- **DB models:** `GatewayTransaction` (COLLECTION/PAYOUT, status, order_id, result_code,
  raw_response, webhook_payload), `Disbursement`, plus existing `PaymentRecord`/`Transaction`/
  `LedgerEntry`. Payment lifecycle: **push → USSD PIN prompt → outcome via webhook + polling**
  (`reconcile_gateway_transactions` beat task, every 5 min).
- **`resultcode 000 / "Wallet push successful"` means the prompt was PUSHED, not that money
  settled.** Real settlement is confirmed by the webhook or a `query_status` poll. Don't treat a
  push-success as paid.
- **Webhook endpoint:** `POST /api/payments/webhooks/selcom/` (GET returns 405 — that's healthy,
  it's POST-only). The app sends Selcom the base64 callback URL; verify it's reachable publicly.
- **⚠️ KNOWN OPEN ISSUE (as of this writing):** the droplet **cannot reach `apigw.selcommobile.com:443`**
  — DNS resolves (`102.222.27.17`) but TCP connect times out (both host and container). Selcom
  almost certainly **IP-whitelists** merchant servers; get **`161.35.167.245` whitelisted with
  Selcom** (and check DigitalOcean cloud-firewall egress). Until then, create-order and the
  reconciler time out (`checked 1, resolved 0`), so payments can't be confirmed regardless of code.
  The `SB…`-prefixed key/vendor look like **sandbox** creds while the base URL is **production** —
  make the key environment and the URL environment agree.
- Diagnose connectivity:
  ```bash
  ssh root@161.35.167.245 'curl -s -o /dev/null -w "connect=%{time_connect}s http=%{http_code}\n" \
    --connect-timeout 15 https://apigw.selcommobile.com/v1/'
  ```

## 8. Driver matching / dispatch (why "no driver found" / "driver online but invisible")

- Matching reads **purely from Redis**, zero SQL: candidate ids from `h3:{hex}:drivers` sets, state
  from `driver:{id}:state` hashes (written by the Java WS gateway on each location ping and by
  go-online `POST /api/users/driver/status/`). Entry point: `dispatch_trip` (Celery) →
  `utils/songa_s1/suggest_driver.py` → `near_by_drivers.py`.
- **Freshness gate:** a driver is excluded if `now - last_update > telemetry_staleness_limit`
  (default **300s**), admin-tunable via `MatchingConfig` (`config/matching.py`). Radius
  `nearby_radius_km` (5km). The rider endpoint `POST /api/trip/nearby-drivers/` uses the same path
  and now returns `last_seen` + `age_seconds` per driver.
- "Online but invisible" = the driver app stopped sending GPS (screen off / OEM power-saving /
  movement-only), so telemetry went stale and the (correct) freshness gate hid them. The DRIVER
  APP needs a heartbeat + background location; do NOT tighten `telemetry_staleness_limit` below
  300s until that ships.
- **Rating is a SOFT preference, not a hard gate** (`suggest_driver.py`): unrated drivers
  (rating `0.0`) and below-cutoff drivers are still matchable (fallback), so a `rating_cutoff` can
  never cause NO_DRIVER_FOUND while a dispatchable driver exists.

## 9. Fraud subsystem (cancellation fraud)

Already built in the backend: `DriverFraudProfile`, `TripAction` telemetry fields, cancellation
fee gate (`trips/utils/cancellation_fraud.py`), co-movement + stalling Celery tasks, and admin
endpoints under `/api/admin/fraud/`. Auto-suspend is behind an admin switch (default OFF).
Suspension is reconciled: `DriverStatusView` go-online enforces `DriverFraudProfile
.is_suspended_for_fraud`. Frontend fraud dashboard (in `songa_admin`) is the remaining piece.

## 10. OTP / SMS (NextSMS)

- OTP: `POST /api/users/generate-otp/`, `POST /api/users/verify-otp/`. A per-number **rate limiter**
  returns `429` on rapid resends (not an error — anti-abuse). `verify-otp 400` = wrong/expired code.
- Delivery via **NextSMS**, sender ID **`SONGA`** (env `NEXT_SMS_SENDER_ID`, wired in the compose
  `environment:` blocks + worker). A successful send returns NextSMS `PENDING_ENROUTE` (accepted &
  queued to carrier) — final handset delivery is async and not logged.
- If OTP "errors" but the backend shows only `200`s, it's client-side or SMS-delivery, not the
  server.

## 11. Auth / JWT (unexpected logouts)

`SIMPLE_JWT`: access 60 min, refresh **30 days**, `ROTATE_REFRESH_TOKENS=True`,
`BLACKLIST_AFTER_ROTATION=True`. A refresh token is **single-use**: each refresh returns a new one
and blacklists the old. Unexpected logouts are almost always the **app discarding the rotated
refresh token** (keeps the old → next refresh is blacklisted → logout), or double-refresh races —
NOT a short server lifetime. Backend needs no change.

## 12. WebSocket groups (Java/Netty gateway)

Broadcast via `broadcast_to_ws_group` (Kafka → gateway). Groups: `user_{rider_id}`,
`driver_{driver_id}`, `trip_{trip_id}`, and **`trip_admin`** (live admin dashboard). Note: unloading
start/end and trip-end payloads are sent identically to rider/driver/admin (only `target` differs).

## 13. Health checks & triage cheatsheet

```bash
# containers + images
ssh root@161.35.167.245 'docker ps --format "{{.Names}}\t{{.Status}}" | grep -E "songa|celery|kafka|watchtower"'
# recent backend errors (ignore internet scanner 404s)
ssh root@161.35.167.245 'docker logs --since 30m root-songa_backend-1 2>&1 | grep -iE "error|traceback|500 " | grep -viE "googlemaps|footprints|wp-json"'
# worker task activity
ssh root@161.35.167.245 'docker logs --since 15m root-celery_worker-1 2>&1 | tail -40'
# migrations state
ssh root@161.35.167.245 'docker exec root-songa_backend-1 ./env/bin/python manage.py showmigrations | grep "\[ \]"'
```

Known past incidents (all fixed): admin trip-delete 500 (`TripAction.__str__` NoneType);
go-online 500 (local `timezone` re-import shadowing); `PrepareTrip` 500 (`number_of_carriers`
undefined); `enforce_account_holds` pgbouncer server-side cursor `InvalidCursorName` (transaction
pooling). Fraud view crash on a missing `DISPATCHER` role. Unapplied `trips 0010` fraud-fields
migration 500ing the fraud endpoint.

## 14. Conventions

- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- Keep `main` and `test` in sync when you intend a deploy: after `git push origin main:test`, do
  `git branch -f test <ref>` locally to avoid confusion.
- The repo also contains `docker-compose.yml` / `docker-compose.prod.yml` which already reference
  the Selcom vars via `${...:-}`, but the **running** compose is `/root/songa-backend-app-v2.yml`
  on the droplet — that's the one to edit for live env changes.
- Repos in this workspace: `songa_backend_revamp_v2` (backend, deploys to server),
  `songa-rider-app` + `songa_driver_app` (Flutter, ship as app builds — not to this server),
  `songa_admin` (Next.js admin dashboard, deploys separately).
