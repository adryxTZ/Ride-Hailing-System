# Spec: driver debt limit, account holds and mobile-money debt pay-in (Phase B, slice 1)

Factory run started 2026-10-21. Spec by the spec-writer agent; owner questions (end) proceed on the stated defaults until the owner says otherwise.

## 1. Goal
A driver whose cash-trip debt on `DRIVER_WALLET` passes the brand's limits is put **on hold**: cannot go online, and is taken offline (never mid-trip). The wallet shows what they owe, the limits and when a hold will start. **Pay now** sends a mobile-money USSD prompt; when the gateway confirms, a balanced ledger transaction reduces the debt and the hold lifts by itself. Finance staff see holds and pay-ins and can place/release a manual hold with a reason (audited). The gateway is a port: `fake` adapter for local/CI, `selcom` adapter compiled and ready for credentials.

## 2. Scope
**In:** fee-policy columns for limits and minimum pay-in (versioned); pure hold rule + `holds` module (automatic DEBT and staff MANUAL holds, independent); online gate, forced offline, 15-min sweep; `payment.Gateway` port (Collect, Status) with `fake` and `selcom`; `payments` module for debt pay-ins (idempotent create, status refresh, untrusted webhook, per-pay-in check job, reconciliation, expiry); ledger post `payin.collected` to new `GATEWAY_CLEARING` account in the same DB transaction as the status change; WS frame `wallet.updated`; driver app wallet card, hold banner/panel, Pay now sheet; admin Finance holds + pay-ins + place/release.
**Out:** rider mobile-money/card fares, payouts, gateway fee accounting and bank sweep, debt waivers, PDF receipts, SMS/push on hold, manual "I paid" declarations.

## 3. Contract
Schemas: `Hold {id, kind DEBT|MANUAL, cause HARD_LIMIT|SOFT_LIMIT_OVERDUE|STAFF, reason?, since}`; `DebtLimits {soft_limit, hard_limit, soft_grace_days, min_payin}`; `Payin {id, status PENDING|COLLECTED|FAILED, failure_code? INSUFFICIENT_FUNDS|REJECTED_BY_USER|EXPIRED|GATEWAY_ERROR, amount, currency, phone, gateway, created_at, completed_at?, expires_at}`; WS `wallet.updated {balance, debt, hold?, payin?}`.
- `GET /driver/wallet` adds `debt`, `limits`, `holds[]`, `soft_hold_at?`, `pay_to_unhold?`, `pending_payin?`; entry kind `payin.collected`.
- `GET /driver/status` adds `holds[]`; `PUT /driver/status` online while held → 409 `DRIVER_ON_HOLD` `details {holds, pay_to_unhold}`; offline always allowed.
- `POST /driver/payins` (Idempotency-Key required, 8–64 chars) body `{amount (whole), phone?}` → 201 Payin, replay 200; errors `IDEMPOTENCY_KEY_REQUIRED`, `VALIDATION_FAILED`, `PAYIN_BELOW_MINIMUM {min}`, `PAYIN_ABOVE_DEBT {max}`, `NOTHING_TO_PAY`, `PAYIN_IN_PROGRESS {payin_id}`, 502 `GATEWAY_UNAVAILABLE`.
- `GET /driver/payins/{id}` → own pay-in or 404.
- `POST /payments/webhooks/{gateway}` public; extracts only the order reference; 200 for a known gateway, 404 unknown gateway; never sets status from the payload.
- `GET /admin/finance/holds` (Finance), `GET /admin/finance/payins?status&driver_id&limit` (Finance), `POST /admin/drivers/{id}/hold` and `/hold/release` (Finance, Operations; reason required; `HOLD_EXISTS`, `NO_MANUAL_HOLD`, `DRIVER_NOT_FOUND`). Release touches MANUAL holds only.
- `FinanceSummary.gateway_clearing`, `DriverBalance.on_hold`.
- All new codes in `backend/i18n/{en,sw}.json`.

## 4. Data
Migration `00026_debt_holds_payins.sql` (NO TRANSACTION): `account_kind` += `GATEWAY_CLEARING`; `fee_policies` += `debt_soft_limit 1000`, `debt_hard_limit 5000`, `debt_soft_grace_days 7`, `payin_min_amount 1000` with CHECK `min>0, soft>=min, hard>=soft, grace>=0`; tables `driver_holds` (one active per kind: partial unique index), `driver_debt_clock (brand, driver, over_soft_since)`, `gateway_transactions` (purpose DRIVER_DEBT, payer, phone, amount, currency, gateway, provider_ref, status, failure_code, idempotency_key unique per payer, checks, last_checked_at, expires_at, created/completed; one PENDING per payer+purpose; pending and recent indexes). Seed `fee_policy.json` gains the four fields. Limits read from the policy in force (account rule, not pinned per trip). No new Redis keys. Env: `NEXA_PAYMENT_GATEWAY` (fake|selcom), `NEXA_FAKE_GATEWAY_DELAY` (5s), `NEXA_SELCOM_BASE_URL/API_KEY/API_SECRET/VENDOR`, `NEXA_PAYMENT_WEBHOOK_BASE_URL`; `selcom` with missing credentials fails fast; `fake` logs a WARN.

## 5. Behaviour
**Rule** `holds.Decide(debt, limits, overSoftSince, now)`: debt < soft → free, clock cleared; debt ≥ soft → clock (kept or now), HoldAt = clock + grace; held if debt ≥ hard (HARD_LIMIT) or now ≥ HoldAt (SOFT_LIMIT_OVERDUE). PayToUnhold = smallest whole amount making it free (overdue: ceil(debt−soft)+1; hard: ceil(debt−hard)+1), clamped to [min, ceil(debt)]. Partial payment that leaves debt ≥ soft does not reset the clock.
**Holds service:** `Evaluate` (advisory lock per driver; upsert/delete clock; place/release/update DEBT hold; audit with system actor; after commit Enforce + WalletChanged; never touches MANUAL); `Enforce` (offline unless on a trip); `Place`/`Release` MANUAL with audit; `Active`; `ListActive` single query. Evaluation runs from a ledger after-commit hook on any `DRIVER_WALLET` leg, on trip terminal status (enforce), and a River sweep every 15 min (batch 500).
**Gateway:** `Gateway{Name, MinAmount, Collect, Status}`. Fake: stateless ref `fake_<ms>_<outcome>`, phone suffix 0001 insufficient funds, 0002 rejected, 0003 never completes (expires), 0004 Collect unavailable, else collected, after Delay; min 1000 TZS. Selcom: HMAC-SHA256 checkout signing, create-order-minimal + wallet-payment, order-status mapping table, `ErrNotConfigured` without credentials.
**Pay-ins:** Create (replay by key; validate min = max(policy, gateway) ≤ amount ≤ ceil(debt); insert PENDING with expires = now+3 min and enqueue check job in the same tx; Collect outside tx; failure → FAILED/GATEWAY_ERROR + 502). Refresh is the only state mover (webhook, check job every 10 s after 8 s, reconcile every 2 min for rows stale > 30 s, batch 100; re-checks FAILED/EXPIRED < 24 h for late success → posted and audited `payin.late_collected`). COLLECTED: conditional update + `PostPayinCollected` in one tx (legs `GATEWAY_CLEARING +a`, `DRIVER_WALLET −a`, idempotent by reference), then hold evaluation + wallet frame.

## 6. Apps and admin
Driver app: `models/wallet.ts`, `services/wallet.ts`, `hooks/useWallet.ts`, `hooks/usePayin.ts` (idle → submitting → pending → success|failed, WS + 5 s polling fallback), WalletCard (debt, limit, warning date, Pay now), `HoldBanner`, `PayNowSheet`, `PayinStatus`, `home/HoldPanel` replacing the online switch. `@platform/ui` + tokens, eased motion, no bounce. Copy EN/SW (wallet.*, hold.*, pay.*) as listed by the spec writer: e.g. "You owe / Unadaiwa", "Your account is on hold / Akaunti yako imesimamishwa kwa muda", "Check your phone and enter your PIN to approve {amount}. / Angalia simu yako na uweke PIN kuidhinisha {amount}.", "Payment received. Thank you! / Malipo yamepokelewa. Asante!".
Admin Finance: HoldsCard (Release for MANUAL only; DEBT shows "Lifts when paid"), PayinsCard (status filter, refresh while pending), "Clearing at gateway" tile, "On hold" chip and "Place hold" on balances; Pricing fee-policy form gets the four fields.

## 7. Tests
Go: rule table tests; holds service (DB) incl. manual hold survives Evaluate with debt 0, on-trip not kicked, sweep places soft-overdue hold; policy validation; fake adapter outcomes and statelessness; selcom signing/mapping (httptest, no network); payments service (idempotency, in-progress, min/above/nothing, success posts one balanced txn and lifts hold, failures post nothing, expiry, webhook cannot fake success, 10 concurrent refreshes → one txn, check worker snoozes); money PostTx/Post parity; httpx (routes declared, 409 on hold, 400 without key, roles).
E2E: `apps/tools/e2e-payin.mjs` (hold from a cash trip, 409, pay-in replay, collected lifts hold, failure suffix, manual hold not lifted by payment, ledger verifies, policy restored); admin e2e extended; driver web e2e extended.

## 8. Acceptance
- [ ] Migration applies/rolls back; seed and new policy versions carry the four fields; invalid combinations refused.
- [ ] Hard limit holds at once; soft overdue held by the sweep without any trip.
- [ ] Held driver cannot go online (409); online driver taken offline; on-trip driver finishes first.
- [ ] DEBT hold lifts by itself; MANUAL never auto-lifts; releasing MANUAL never lifts DEBT.
- [ ] Pay now pushes through the Gateway port; fake completes after the delay; suffixes 0001–0004 behave as documented; app shows each state in EN and SW.
- [ ] Webhook never decides status; expired pay-ins end FAILED/EXPIRED.
- [ ] Each COLLECTED pay-in posts exactly one balanced `payin.collected` in the same tx; hash chain verifies.
- [ ] Replayed key returns the original pay-in without a second push; one pending pay-in per driver.
- [ ] `selcom` compiles with signing/mapping tests; server refuses `selcom` without credentials.
- [ ] Admin Finance shows holds, pay-ins, clearing; place/release ask for a reason and appear in Audit; system changes appear with a system actor.
- [ ] Driver Home shows the hold panel instead of the switch; wallet shows debt, limit, warning date, Pay now; eased motion.
- [ ] OpenAPI + both clients regenerated; gofmt, vet, lint, tsc, brand grep pass; `doc.go` everywhere; no literal brand or colour.
- [ ] p95 and queries per request recorded for wallet, status and pay-in create.

## 9. Owner questions (defaults in use)
1. Limits: soft 1,000, hard 5,000, 7 grace days, minimum pay-in 1,000 TZS (Songa's values), one set per brand.
2. Partial payment does not restart the grace days.
3. Platform bears the Selcom collection fee; driver credited in full.
4. Late gateway success after expiry is credited automatically within 24 h and audited.
5. Drivers may pay from another mobile-money number.
6. In-app notice only (no SMS/push on hold) for the pilot.
7. Selcom API key, secret, vendor id and a whitelisted webhook URL per environment are needed before UAT.

## Factory log
- 2026-10-21 spec written; build started.
- 2026-10-21 build done; tests: Go (rule, holds, payments incl. 10-way race, fake and Selcom adapters), e2e-payin, e2e-web-hold, e2e-admin all pass.
- Round 1 reviews: security CHANGES (lost reply on ambiguous gateway error; no push rate limit; unbounded webhook work), ux CHANGES (22, main: staff-hold wording, pending prompt lost on close, premature "check your phone", two debt figures), ui CHANGES (15, main: literal scrim colour, hand-rolled buttons, journal wrapping/paging), code CHANGES (6, main: status error never expires, lost reply, lock ordering). All fixed except: IP rate limit on the webhook route (work is now bounded and deduplicated), gateway minimum read from the server (hint instead). Backend regression tests added for the lost reply and the push limit.
- Round 2 reviews requested.
- 2026-10-21 later: dashboard sign-in switched to email + password (`POST /auth/staff/login`, bcrypt, 5 failures lock 10 min; `nexa staff add --email --password`; Staff page sets credentials); side navigation replaced the pill bar and icon rail; Docker: `frontend/admin/Dockerfile`, `apps/Dockerfile`, `infra/compose.yml`, `infra/up.sh` (written, build not yet run). Approver launched.
- Approver: ESCALATE on four points, resolved as follows.
  1. Measurements (local, Postgres in Docker, n=200/100): `GET /driver/wallet` p95 4.7 ms, 6 queries (entries, balance, debt, clock, active holds, pending pay-in; limits from the pricing cache); `PUT /driver/status` p95 1.9 ms (offline) / +3 queries online (eligibility, debt, holds); `POST /driver/payins` ~3 ms to the validation answer, 6 queries plus the gateway call (replay lookup, debt, push limits, insert + job in one tx, read-back). All under the 10 ms EXPLAIN threshold.
  2. Driver-app copy is English only: the driver app has no translation layer yet (every screen is English; server messages are localised). Deferred to an "app i18n (EN/SW)" slice, listed in the roadmap; the owner decides its priority.
  3. `doc.go` added to `providers/payment`, `fake`, `selcom`; the whole backend now passes `golangci-lint run ./...` (the pre-existing errcheck/unused findings were fixed too).
  4. Reviewer item 5 fixed: the hold evaluation reads the debt on its own locked transaction (`Ledger.DriverDebtIn`).
  Outcome: accepted by the builder with the Swahili deferral flagged to the owner; nothing committed (owner's call).
