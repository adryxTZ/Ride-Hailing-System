# Taxi roadmap: what is done and what remains

Status: 2026-10-20, after the taxi-only decision (ADR 0009). Replaces the multi-service phase table in `01-platform-redesign.md` §D9/§D11 for planning purposes.

## Done

| Area | What exists and is tested |
|---|---|
| Foundation | Go service (`nexa`), Postgres + Redis, migrations, brand/theme as data, OTP sign-in with EdDSA tokens, OpenAPI contract and generated clients, Expo monorepo, dev scripts |
| Booking | Quote with road route and ETA (OSRM), ride types Bajaji / Standard / XL, up to 3 stops, place search and reverse names, fixed fares (town – airport), "Who is riding?" for someone else, fare split shown before booking |
| Matching | Live supply in Redis (GEO index, H3 cells, area cache), batched min-ETA assignment, sequential offers (20 s), never re-asks a driver, re-dispatch on driver cancel, no-driver timeout |
| The ride | Live trip screen both sides over websockets, driver location on the rider's map, turn-by-turn directions, ride-in-progress takes over the app, cancel with reasons, cash payment step |
| After the ride | Receipt (fare, booking fee, VAT, levy), rider Activity and driver History, driver earnings and wallet, double-entry hash-chained ledger |
| Talking | In-trip chat with quick replies, ratings both ways, push (server done; phones need a development build) |
| Drivers | In-app registration (details, vehicle, 6 photos), staff review API (approve, reject with reason, suspend, reinstate, per-document review, audit log), only approved drivers and vehicles go online |
| Capacity | Measured: one API process covers 100,000 rides/day with ≥ 5× headroom (`05-capacity-plan.md`) |

## Remaining

### Phase A: Ride features riders expect: DONE 2026-10-20 (promo codes left as optional)
Built and tested: scheduled rides (Later → time picker → Upcoming in Activity → matching 15 min before), pickup waiting fee (free minutes, per-minute, capped 3% per LATRA), cancellation and no-show fees (free 2 minutes, then the ride type's fee; 80% to the driver; carried to the rider's next trip), surge per H3 area capped per ride type (Standard 1.08 = LATRA's top band), Safety (share live link page, SOS texting trusted contacts + staff queue, call 112), Boda ride type and Boda package delivery, LATRA-based prices, 1% levy, fares rounded to 50 TZS.

Original plan:
1. **Schedule a ride** (the "Schedule" tile now says "coming soon"): `scheduled_for`, dispatch starts before the time, reminders.
2. **Pickup waiting fee:** the clock starts at "arrived"; after the grace minutes the waiting rate is added (engine support exists).
3. **Cancellation fee** when the rider cancels late or doesn't show; part goes to the driver.
4. **Surge pricing** per H3 area from open requests vs free drivers, capped and locked at quote.
5. **Safety:** share trip link, SOS button, trusted contacts (the Safety pill is a placeholder).
6. **Boda (motorbike) ride type**, if wanted (one catalogue entry + a tariff).
7. Promo codes / referral (optional).

### Phase B: Money (≈ 2–3 weeks; needs Selcom credentials)
1. Mobile-money and card fares (Selcom checkout), payment states PENDING → COLLECTED / CASH_CONFIRMED / FAILED.
2. Driver debt limit and holds (cash trips leave the platform share as debt; above the limit the driver cannot go online until paying).
3. Driver pays debt by mobile money (USSD push); payouts of earnings from digital trips.
4. Reconciliation job, receipts as PDF / email.
5. Owner decisions: LATRA levy rate, booking fee, commission and VAT confirmation, real tariffs per ride type.

### Next spec: operating areas (Kalen, 2026-10-21)
The Platform must run in several countries, each divided the way that country divides itself (regions, counties, states), with the currency, taxes, regulator and payment gateway chosen per country, and prices, surge, reports and dashboards scoped to an operating area the admin selects. Money already carries its currency on every amount and ledger account; the missing layer is `operating areas` (country → region) on tariffs, policies, trips and the admin views. To be specced through the factory after the debt-holds slice.

### Phase B progress (2026-10-21)
- Slice 1 done through the factory: driver debt limits, account holds, mobile-money debt pay-ins (fake gateway + Selcom adapter), Finance holds/pay-ins, Pricing debt fields. Spec and log: `DOC/specs/2026-10-21-debt-holds-payins.md`.
- Deferred, needs the owner's priority: **driver/rider app translations (EN/SW)**; the apps have no translation layer yet, all screen copy is English.
- Slice 2 done 2026-10-23 (spec `DOC/specs/2026-10-21-fares-payouts-receipts.md`):
  - **Rider fares by mobile money:** an automatic USSD prompt when the ride ends; the rider can retry (3 prompts per trip, another number allowed) or pay cash. The driver can confirm cash only when no prompt is open. The trip completes in the same transaction that records the money, and the database refuses a COMPLETED trip without a settled fare. Money that arrives after the trip was settled is owed back to the rider (RIDER_REFUNDS, audited).
  - **Receipts:** one per completed trip, issued inside the completing transaction. Numbers are gapless per year (`2026-000123`), and the database refuses any change once issued. Both apps get JSON, and an A4 PDF in English or Swahili comes through a 15-minute signed link. The PDF is drawn by `kernel/pdf`, with no dependencies.
  - **Driver payouts (cash-outs):** paid to the account phone only, with a minimum and an approval switch in the fee policy, and Finance approves each request unless approval is automatic. The ledger moves only on PAID, an unclear gateway reply never fails a payout, and one still pending after 30 minutes is flagged as stuck. Selcom's Business API is wired (RSA-signed transfers, wallet chosen from the number's prefix), alongside the fake gateway.
  - **Admin:** Finance page Cash-outs card (approve or reject with a reason), "Cash-outs in progress" and "Owed to riders" tiles, mobile-money list for both kinds, payout rules on the policy form, and payment status with a receipt PDF on the trip page.
  - **Tests:** `e2e-fares.mjs`, `e2e-payouts.mjs`, `shot-fares.mjs`, `shot-payouts.mjs`, plus Go tests for trips, money, receipts, payouts and both payout adapters.
- Remaining in B: card fares (need Selcom card credentials), refunds paid out of the app, reconciliation to the bank (gateway fees, bank sweep), and fiscal receipts if TRA requires them.

### Phase C: Compliance and trust (≈ 1–2 weeks; needs LATRA CeTS access)
1. LATRA: register vehicles and drivers on approval, submit each trip, daily report, retries.
2. Document expiry reminders (licence, insurance) and automatic suspension when expired.
3. Fraud checks v1 (fake GPS, self-booking, cancel abuse), PDPA consent and data retention.
- Spec: `DOC/specs/2026-10-23-compliance-trust.md` (slices S1 regulator, S2 document expiry, S3 fraud v1, S4 PDPA).
- S1 done 2026-10-23, kept generic per country (LATRA is only Tanzania's adapter). Payment gateways were made per country at the same time.
- S2 done 2026-10-23: document types are set per country by staff (mandatory, optional or not asked; expiry tracked; expired blocks or not), with renewals, reminders at 30/7/1 days and an automatic hold.

### Phase D: Admin dashboard: DONE 2026-10-20 (branding page, refunds and payouts left for Phase B)
Next.js in `frontend/admin` with the generated client, phone sign-in for staff, and pages gated by role (Super Admin, Operations, Finance, Support, Driver Review):
- **Overview:** today's figures, trips by hour, a needs-attention list and the money position.
- **Live:** map of drivers and open requests, plus the surge areas.
- **Trips:** search and filters, with a drawer showing route, timeline, fare lines and staff cancel.
- **Drivers:** review queue with documents, approve / reject / suspend / reinstate.
- **Riders:** search, block and unblock.
- **Pricing:** tariffs per ride type, the fee policy (publishing creates a new version) and fixed fares.
- **Finance:** ledger totals, driver balances, the journal and the hash-chain check.
- **Safety:** SOS queue.
- **Audit log.**
- **Staff:** accounts and roles.

Every staff write asks for a reason and is audited.
- Design: light and dark themes, with brand colour from `/app/config`.
- Test: `node apps/tools/e2e-admin.mjs` opens every page and verifies the ledger.
- Screenshots: `node apps/tools/shot-admin.mjs <paths>`.

Left for later: branding/theme editor, refund / set price (with Phase B payments), payouts (Phase B).

### Phase E: Release (≈ 1–2 weeks)
1. Development and store builds with EAS (needed for push), app icons and splash per brand, Android and iOS signing.
2. Production routing and places: self-hosted OSRM + Nominatim, or Google keys.
3. Hosting: separate UAT and production, managed Postgres, TLS, CI deploys with migrations, monitoring and alerts, backups.
4. Load test in the target environment, then a pilot with about 20 drivers, then launch.

## Order recommended
A (rides) → D (admin, so operations can run the review queue and prices) → B (money) → C (compliance) → E (release and pilot). B and C depend on external credentials; request Selcom and LATRA access now so they are ready when needed.

## What is left (Kalen, 2026-10-23: "implement this all")
| Phase | What's left |
|---|---|
| **B: Money** | Card payments for fares (needs Selcom card credentials), refunds paid out from the app, bank reconciliation, and fiscal receipts if TRA requires them |
| **C: Compliance and trust** | S3 fraud checks (fake GPS, people booking themselves, cancellation abuse, a review queue) and S4 data protection (consent versions, data retention, export and delete requests) |
| **D: Dashboard leftovers** | The branding page (name, colours, logo, icons), and refund or set-price on a trip |
| **E: Release** | Store builds with icons, splash and signing; production maps and address search; hosting with separate test and live servers, automatic deploys, monitoring and backups; a load test; a pilot with about 20 drivers |
| **Pricing profiles** | Default and scoped profiles (region, district, place, geofenced zone), active or not, extra charges (toll, parking, levy …), an immutable pricing record on every trip, assignment history "as of" a time, and a geofencing page. Spec: `DOC/specs/2026-10-23-pricing-profiles-geofences.md` |
| **Smaller items** | Fees per vehicle type, safety alerts limited by country, a region picker in the driver app, testing the one-command Docker run, recording response-time measurements |

Progress 2026-10-23: branding page done (Settings › Branding: identity, presets, Color Hunt import, custom colours, dark mode, logos and icons, contrast checks, publish and rollback). Pricing profiles S1 (profiles, charges, pinned per-trip record) and S2 (geofences: districts, zones, places, drop-off and route charges) done; S3 done: Pricing history (timeline of which profile applied where, who and why; "as of" any past time, rebuilt from history; CSV export of trip pricing records) and the trip simulator (fares, lines and split per ride type, with a draft profile preview; nothing saved). Compliance S3 (fraud v1) done: fake-GPS strikes counted in Redis on the ping path, self-booking checks at completion (shared app install, passenger or receiver phone is the driver's), daily cancellation scan per country, a driver is never offered their own ride, admin Trust › Fraud flags review queue (dismiss or confirm, audited, frozen once reviewed). Compliance S4 (data protection) backend done: versioned consent per app with the gate on booking, going online and submitting, export and delete requests handled by staff (deletion anonymises and is blocked while a ride or money is open), daily retention job with 12 rules; app consent and privacy screens (both apps) and admin Trust › Privacy (requests, blockers, retention) done. Dashboard trip actions done: set price before payment (MANUAL_PRICE line, split re-checked, no waiting fee on top) and refunds after completion (platform- or driver-borne, owed to the rider until Finance pays, idempotent). Smaller items done: SOS alerts limited by the staff member's country and region; driver app region picker at sign-up (GET /driver/areas). Fees per vehicle type: covered by per-class tariffs and charges limited by class (spec S1 out-of-scope note); a per-class commission needs an owner decision. Release tooling done (one-command Docker run verified, UAT/production compose with Caddy, backups with a restore drill, monitoring and alerts, CI/CD workflows, EAS profiles, brand asset export, k6 load test smoke, p95 baseline in `DOC/perf/`); remaining for release needs accounts and servers (see `DOC/release/launch-checklist.md`).

Build order: pricing profiles and geofencing → branding → fraud (C S3) → data protection (C S4) → dashboard leftovers → smaller items → money leftovers → release tooling. Items needing outside credentials or accounts (Selcom cards, TRA, store accounts, servers, the pilot) are built against fake adapters and documented, then switched on when the credentials arrive.
