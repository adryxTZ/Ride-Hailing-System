# Launch checklist

Purpose: every account, credential, setting and asset the Platform needs before a brand goes live, with where each value goes.
Date: 2026-10-23

How to read an item: `Owner` is a placeholder to fill in; `Where` says where the value lives.
"Env" means the backend environment of the target stack (UAT and production each have their own),
held as GitHub Environment secrets (`uat`, `production`) or the host's env file. Never commit values;
never paste secrets into issues, chats or this file. Placeholders: `<brand>`, `<domain>`, `<country>`.

## Known blockers (need outside accounts or missing code)

| # | Blocker | Why it blocks | Owner |
|---|---|---|---|
| 1 | **Production SMS adapter not built.** The backend only has a log sender for OTP. | No rider or driver can sign in without SMS OTP. Needs a provider account, sender ID approval and a new adapter behind the messaging port. | `<owner>` |
| 2 | Selcom checkout merchant account (pay-in) | No mobile money payments | `<owner>` |
| 3 | Selcom Business account and registered RSA key (payouts) | Drivers cannot be paid out | `<owner>` |
| 4 | Transport regulator credentials per country (Tanzania: LATRA CeTS) | Trips cannot be reported where reporting is mandatory | `<owner>` |
| 5 | Apple Developer Program (organisation, needs D-U-N-S) | No iOS store or TestFlight builds; D-U-N-S can take 1-2 weeks | `<owner>` |
| 6 | Google Play Console account | No Android store build | `<owner>` |
| 7 | SMS sender ID registration with the national regulator | Some countries reject unregistered sender IDs | `<owner>` |
| 8 | Domain ownership and DNS access | No TLS, no public API | `<owner>` |

## 1. Payments (Selcom)

### Checkout, pay-in
- [ ] Merchant account opened, KYC done, settlement account linked. Owner: `<owner>`. Where: Selcom portal.
- [ ] `NEXA_SELCOM_BASE_URL` (production endpoint, not sandbox). Owner: `<owner>`. Where: Env.
- [ ] `NEXA_SELCOM_API_KEY`. Owner: `<owner>`. Where: Env secret.
- [ ] `NEXA_SELCOM_API_SECRET`. Owner: `<owner>`. Where: Env secret.
- [ ] `NEXA_SELCOM_VENDOR` (vendor / till id). Owner: `<owner>`. Where: Env.
- [ ] `NEXA_PAYMENT_WEBHOOK_BASE_URL` = `https://api.<domain>`; callback URL whitelisted with Selcom. Owner: `<owner>`. Where: Env and Selcom portal.
- [ ] Test pay-in of a small amount on production, refund it, confirm in admin **Finance**. Owner: `<owner>`.

### Business, payouts
- [ ] Selcom Business account opened and funded. Owner: `<owner>`. Where: Selcom portal.
- [ ] `NEXA_SELCOM_PAYOUT_BASE_URL`. Owner: `<owner>`. Where: Env.
- [ ] `NEXA_SELCOM_PAYOUT_API_KEY`. Owner: `<owner>`. Where: Env secret.
- [ ] RSA key pair generated; public key registered with Selcom; private key mounted on the host. Owner: `<owner>`. Where: file on host, path in `NEXA_SELCOM_PAYOUT_KEY_FILE`.
- [ ] One payout to a staff number succeeds and shows in **Finance**. Owner: `<owner>`.

### Cards
- [ ] Card acquiring enabled on the Selcom account. Owner: `<owner>`. Where: Selcom portal.
- [ ] `card_payments` added to `NEXA_FEATURES` only after a test card payment passes. Owner: `<owner>`. Where: Env (comma-separated list).

## 2. Transport regulator (per country)

Regulators are per-country ports chosen by configuration, not hard-coded. Repeat this block per launch country.

- [ ] Regulator account and API access for `<country>`. Owner: `<owner>`. Where: regulator portal.
- [ ] Tanzania, LATRA CeTS: `NEXA_REGULATOR_LATRA_ADAPTER=cets` (never `fake` in production). Owner: `<owner>`. Where: Env.
- [ ] `NEXA_REGULATOR_LATRA_BASE_URL`. Where: Env.
- [ ] `NEXA_REGULATOR_LATRA_JWT_ISSUER`, `NEXA_REGULATOR_LATRA_JWT_AUDIENCE`, `NEXA_REGULATOR_LATRA_JWT_KID`. Where: Env.
- [ ] Signing key: `NEXA_REGULATOR_LATRA_JWT_KEY_FILE` points at the private key on the host. Where: file on host.
- [ ] `NEXA_REGULATOR_LATRA_JWKS_URI` published and reachable by the regulator. Where: Env.
- [ ] Test submission accepted; status green in admin **Compliance > Regulator**. Owner: `<owner>`.
- [ ] Other countries: adapter chosen, credentials in Env, test submission accepted. Owner: `<owner>`.

## 3. Messaging

- [ ] SMS provider account with balance and delivery reports. Owner: `<owner>`. Where: provider portal; credentials in Env secret.
- [ ] Sender ID `<brand>` registered and approved. Owner: `<owner>`.
- [ ] Production SMS adapter built, tested, selected in Env (blocker 1). Owner: `<owner>`.
- [ ] `NEXA_OTP_TEST_CODE` and `NEXA_OTP_TEST_NUMBERS` empty in production (store-review numbers only, if used). Where: Env.
- [ ] `NEXA_EXPO_ACCESS_TOKEN` (Expo robot token). Owner: `<owner>`. Where: Env secret.
- [ ] FCM server credentials uploaded to EAS for both apps. Owner: `<owner>`. Where: EAS credentials.
- [ ] APNs key uploaded to EAS for both apps. Owner: `<owner>`. Where: EAS credentials.
- [ ] Push received on a real Android and iPhone for trip offer and trip status. Owner: `<owner>`.

## 4. Routing and places

- [ ] Self-hosted OSRM with the country extracts (`infra/deploy/maps/prepare.sh`), started with the `maps` compose profile of `infra/deploy/compose.yml`. Owner: `<owner>`. Where: host.
- [ ] `NEXA_OSRM_URL`. Where: Env.
- [ ] Self-hosted Nominatim, same profile. `NEXA_NOMINATIM_URL`. Where: Env.
- [ ] Or Google routing / places keys, server-side only, restricted by IP. Where: Env secret.
- [ ] `backend/brands/<brand>.json` has `providers.routing` set to `"osrm"` when self-hosting. Owner: `<owner>`.
- [ ] Route and ETA spot-check on 5 known trips in the launch area. Owner: `<owner>`.

## 5. Mobile accounts and builds

- [ ] Apple Developer Program, organisation account, D-U-N-S number. Owner: `<owner>`.
- [ ] Google Play Console account, developer identity verified. Owner: `<owner>`.
- [ ] Expo / EAS account (organisation); one EAS project per app per brand. Where: `apps/brands/<brand>.json` (`eas_owner`, `rider.eas_project_id`, `driver.eas_project_id`); `BRAND` set as an EAS environment variable on both projects; `EXPO_TOKEN` as a GitHub secret.
- [ ] Bundle ids / package names set per brand. Where: `apps/brands/<brand>.json`.
- [ ] Android upload key and iOS distribution certificate + provisioning profiles created and held by EAS credentials (never in the repository). Owner: `<owner>`.
- [ ] Production builds for rider and driver apps on TestFlight and Play internal testing. Owner: `<owner>`.
- [ ] EAS Update channels: `prod` for store builds, `uat` for pilot (preview) builds, as set in each app's `eas.json`. Owner: `<owner>`.

## 6. Domains, TLS, servers

- [ ] Domain `<domain>` registered; DNS access for ops. Owner: `<owner>`.
- [ ] DNS records: `api.<domain>`, `uat-api.<domain>`, `admin.<domain>`, `uat-admin.<domain>`, `<domain>` and `www.<domain>` (website). Where: DNS provider.
- [ ] TLS by Caddy automatic Let's Encrypt; ACME email `<ops-email>` set. Where: Caddy config.
- [ ] Separate UAT and production hosts (no shared database, Redis or keys). Owner: `<owner>`.
- [ ] Managed Postgres with PostGIS, point-in-time recovery on, retention `<days>`. Where: database URL in Env secret.
- [ ] S3-compatible bucket for backups; an `age` key pair for backup encryption: public key in `backup.env` on each host, private key only in the password manager. Where: `infra/deploy/env/backup.env.example`.
- [ ] Restore drill done once on UAT (see `infra/README.md`, Backups). Owner: `<owner>`.

## 7. CI/CD and access

- [ ] GHCR access: deploy hosts can pull images (read-only token). Where: host registry login.
- [ ] GitHub Environments `uat` and `production` created; `production` requires named reviewers. Where: repo settings.
- [ ] Secrets above loaded per Environment; nothing in workflow files. Owner: `<owner>`.
- [ ] Brand-name guard, lint and tests green on the release commit. Owner: `<owner>`.

## 8. Monitoring

- [ ] Grafana Cloud stack (or self-hosted Prometheus + Grafana) scraping the backend. Owner: `<owner>`.
- [ ] Panels: API p95 and 5xx, dispatch match rate and time to accept, job queue failures, Postgres and Redis health.
- [ ] Alerts: p95 > 500 ms for 15 min, 5xx > 2 % for 10 min, job failures, disk > 80 %, backup missed. Where: Grafana alerting to `<on-call channel>`.
- [ ] Sentry DSN (optional) for apps and backend. Where: Env and app build config.
- [ ] Daily `nexa verify-ledger` scheduled and its result alerted. Owner: `<owner>`.

## 9. Staff and operations

- [ ] First Super Admin created on production: `nexa staff add <phone> --name "<name>" --roles "Super Admin" --email <email> --password <temporary-password>`; change the password at first login. Owner: `<owner>`.
- [ ] Other staff added with least-privilege roles (Ops, Finance Officer, Support). Where: admin **Staff**.
- [ ] Launch area configured and left inactive until go-live. Where: admin **Areas**.
- [ ] Tariffs per ride type and Package published and checked. Where: admin **Pricing**.
- [ ] Support line `<phone>`, support email and SOS escalation rota staffed. Owner: `<owner>`.

## 10. Legal and store listings

- [ ] Terms of service for riders and drivers. Owner: `<owner>`. Where: website and in-app links.
- [ ] Privacy policy. Where: `https://<domain>/privacy`.
- [ ] Data protection registration with the national authority for each country. Owner: `<owner>`.
- [ ] App Store privacy labels and Play data safety form. Owner: `<owner>`.
- [ ] Store listings: name, description, screenshots (phone sizes per store), support URL, marketing URL. Owner: `<owner>`.
- [ ] Review account for Apple / Google reviewers (test number limited to that account). Owner: `<owner>`.

## 11. Brand assets

- [ ] App icon 1024 x 1024 px, no transparency for iOS. Where: `apps/brands/<brand>`.
- [ ] Splash image and background colour. Where: `apps/brands/<brand>`.
- [ ] Theme published in admin **Settings > Branding**. Owner: `<owner>`.
- [ ] Assets exported with `node apps/tools/brand-assets.mjs` and committed to the brand folder. Owner: `<owner>`.
- [ ] Brand runtime file `backend/brands/<brand>.json` reviewed (name, support contacts, providers). Owner: `<owner>`.

## Go-live day runbook

Run in order; do not start a step until the previous one is green.

| # | Step | Who | Done when |
|---|---|---|---|
| 1 | Tag the release (`v<x.y.z>`) on the approved commit | `<owner>` | CI builds and pushes the image to GHCR |
| 2 | Approve the production deploy in the `production` Environment | `<reviewer>` | Workflow starts |
| 3 | Migrations run as part of the deploy | automatic | Deploy log shows migrate OK; services healthy |
| 4 | Smoke test: health endpoint, staff login, OTP sign-in, fare quote, one staff trip with mobile money, payout dry check, regulator submission | `<owner>` | All pass; `verify-ledger` OK |
| 5 | Submit rider and driver store builds for review (or promote reviewed builds to production) | `<owner>` | Builds live or in review |
| 6 | Open the area: set the launch area active in admin **Areas** | `<owner>` | Riders in the area can request trips |
| 7 | Watch Grafana and the support queue for the first 2 hours | on-call | No rollback trigger fired |

Rollback for any step: see `pilot-plan.md` section 10.
