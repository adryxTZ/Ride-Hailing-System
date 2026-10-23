# Load test (k6)

DOC/01 §D11 target: **600 online drivers pinging every 10 s + about one quote per second
(a share of them booked), p95 < 150 ms, under 1 % errors**, API RAM < 400 MB. Run it on UAT
before the pilot (Phase E.4). Never point it at production: it creates users and trips and
moves presence.

| File | Purpose |
|---|---|
| `platform.js` | Scenarios `drivers` (one VU per driver, `POST /driver/location` every 10 s) and `riders` (`POST /quotes` at a fixed rate; `BOOK_RATIO` of them `POST /trips` then cancel). Setup signs everyone in by OTP and puts drivers online; teardown takes them offline. |
| `lib.js` | Shared helpers: OTP sign-in (uses `dev_code` when the server echoes it), synthetic phones and positions. |
| `provision-drivers.sh` | Creates the approved synthetic drivers (`nexa driver provision`), one vehicle each across Boda/Bajaji/Standard/XL. |
| `results/` | `--summary-export` JSON output (git-ignored). |

Settings (`-e NAME=value`): `API`, `DRIVERS` (600), `RIDERS` (20), `DURATION` (10m),
`PING_INTERVAL_S` (10), `QUOTES_PER_S` (1), `BOOK_RATIO` (0.3), `DRIVER_PREFIX` (+2557119),
`RIDER_PREFIX` (+2557129), `OTP_CODE` (0000, used only when no code is echoed).

## Running against UAT

1. **Sign-in for synthetic numbers.** The test signs in 620 numbers. On UAT, for the
   duration of the test only, set `NEXA_OTP_DEV_ECHO=true` in `api.env` and restart the API
   (`docker compose -f compose.yml -f compose.uat.yml up -d api`). Turn it off afterwards.
   OTP requests are limited to one per number per 60 s, so wait a minute between runs.
2. **Drivers.** On the UAT host, from `/opt/platform`:
   ```
   NEXA="docker compose -f compose.yml -f compose.uat.yml --profile ops run --rm -T seed" \
     COUNT=600 ./loadtest/provision-drivers.sh
   ```
   (copy `infra/loadtest/` to the host first, or run it from a checkout that can reach the UAT
   database). Each provision is one short container run, about 10 minutes for 600.
3. **Run** from a machine outside the host (the load generator must not compete with the API):
   ```
   docker run --rm -i -v "$PWD/infra/loadtest:/lt" grafana/k6:1.3.0 run \
     -e API=https://uat-api.<domain>/v1 --summary-export=/lt/results/uat-$(date +%F).json /lt/platform.js
   ```
4. **Watch** in Grafana while it runs: API p95 and 5xx (Caddy), container memory of `api`
   (D11 budget: < 400 MB at 600 drivers), River queue depth, Postgres connections.
5. **Record** the summary in `DOC/perf/<date>-uat-load.md`: thresholds, p95 per endpoint,
   peak API memory, and any errors with their codes.

k6 exits non-zero when a threshold fails, so the same command can gate a CI job later.

## Smoke run on a laptop

```
COUNT=20 ./infra/loadtest/provision-drivers.sh      # uses backend/bin/nexa and the dev database
docker run --rm -i -v "$PWD/infra/loadtest:/lt" grafana/k6:1.3.0 run \
  -e API=http://host.docker.internal:8080/v1 -e DRIVERS=20 -e RIDERS=5 -e DURATION=30s /lt/platform.js
```

Set `-e BOOK_RATIO=0` when real phones are online as drivers on the same backend, or they
will receive offers. Results from 2026-10-23: `DOC/perf/2026-10-23-baseline.md`.
