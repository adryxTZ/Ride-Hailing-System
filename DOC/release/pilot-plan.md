# Pilot plan: 20 drivers before launch

Purpose: prove the Platform end to end with real drivers, riders and money in one area before public launch.
Date: 2026-10-23

Owners are placeholders (`<owner>`); fill them in at the week 0 kick-off. Brand-specific values use
`<brand>`, `<domain>`, `<city>`, `<area>`.

## 1. Scope at a glance

| Item | Pilot value |
|---|---|
| Drivers | ~20, recruited and onboarded by hand |
| Riders | ~50: staff plus a friends-and-family invite list |
| Ride types | Boda, Bajaji, Standard, XL, and Package (Boda only, receiver required) |
| Area | One operating area in `<city>` (`<area>`), all others inactive |
| Hours | 07:00-21:00 local, staffed support the whole window |
| Payments | Cash and mobile money (Selcom pay-in); cards off (`card_payments` not in `NEXA_FEATURES`) |
| Builds | Rider and driver apps from the preview / internal store build (TestFlight internal, Play internal testing) |
| Environment | Production stack, production data, real money in small amounts |
| Length | 2 weeks plus 1 prep week |

## 2. Driver recruitment

### Vehicle mix (target 20)

| Ride type | Drivers | Also does Package | Notes |
|---|---|---|---|
| Boda | 8 | yes, all 8 | Highest volume; Package needs a Boda with a box or bag |
| Bajaji | 4 | no | |
| Standard | 6 | no | Saloon or hatchback, 4 seats |
| XL | 2 | no | 6+ seats; low volume, tests the class filter |

### Where to recruit

- Existing boda stages and taxi ranks inside `<area>` (pickup ETA depends on drivers living or working there).
- Referrals from staff and from the first drivers onboarded.
- Fleet owners with 2-3 vehicles (one contact, fast document collection).

### Criteria

- [ ] Valid driving licence for the vehicle class; regulator badge or permit where the country requires one.
- [ ] Vehicle registration, insurance and inspection current (expiry later than pilot end plus 30 days).
- [ ] Android 10+ or iOS 16+ phone with data, GPS and a working camera.
- [ ] Mobile money number in the driver's own name (used for payouts).
- [ ] Available at least 5 days a week for 6+ hours inside pilot hours.
- [ ] Agrees to the daily feedback message and one end-of-pilot interview.

### Incentives

| Incentive | Value | Paid when |
|---|---|---|
| Commission waiver | 0 % commission for the pilot | Every trip, through the normal payout run |
| Online-hours guarantee | `<amount>` per day with 6+ online hours and acceptance >= 80 % | Weekly, as a manual adjustment in Finance |
| Completion bonus | `<amount>` for finishing the 2 weeks | End of week 2 |
| Founding-driver status | Priority in launch onboarding, branded gear | At launch |

## 3. Driver onboarding checklist (per driver)

Copy this block per driver into the pilot tracker.

- [ ] Account created: OTP sign-in in the driver app with the driver's own number.
- [ ] Documents uploaded in the app (licence, ID, registration, insurance, photo).
- [ ] Documents reviewed and approved in admin **Drivers** and **Compliance > Documents** (KYC queue).
- [ ] Vehicle approved with the right ride type; Boda drivers also enabled for Package.
- [ ] Payout number verified (name on the mobile money account matches the driver).
- [ ] Driver app installed from the preview / internal store build; version noted.
- [ ] Location permission "always" granted; battery optimisation off for the app.
- [ ] Test trip completed with a staff rider: request, accept, arrive, start, finish, rating.
- [ ] Briefed on cash vs mobile money: who collects, how the trip screen shows it, what to do if a payment fails.
- [ ] Briefed on SOS (button location, what happens when pressed) and the support line `<phone>`.
- [ ] Added to the pilot driver chat group.

## 4. Rider side

| Group | Count | How invited |
|---|---|---|
| Staff | ~15 | Direct link to the internal store build |
| Friends and family | ~35 | Invite list kept by `<owner>`; each gets the build link and a one-page guide |

- [ ] Riders sign in by real SMS OTP (needs the production SMS adapter, see launch blockers). Do not add
      pilot numbers to `NEXA_OTP_TEST_NUMBERS`; keep `NEXA_OTP_TEST_CODE` empty in production.
- [ ] Each rider does at least 3 trips per week, mixing ride types and at least one Package.
- [ ] At least 10 riders pay by mobile money, the rest by cash.
- [ ] Riders report issues through in-app support or the pilot chat with a trip id.

## 5. Operating area and hours

- [ ] Only `<area>` is active in admin **Areas**; every other area stays inactive.
- [ ] Area boundary covers the drivers' home stages plus main rider destinations.
- [ ] Tariffs for every ride type and Package published and checked against a manual fare calculation.
- [ ] Pilot hours 07:00-21:00; outside hours the area stays active but support is on call only.

## 6. Success metrics

Measured daily, judged on the 7-day rolling value at the end of week 2.

| Metric | Target | Where to read it |
|---|---|---|
| Match rate (requests that get a driver) | >= 85 % | Admin dashboard home, Trips |
| Time to accept (offer to accept, p50) | <= 15 s | Grafana dispatch panel |
| Pickup ETA accuracy (actual vs quoted) | within +/- 2 min on 80 % of trips | Grafana dispatch panel |
| Rider cancellation rate after match | <= 10 % | Trips |
| Driver cancellation rate after accept | <= 5 % | Trips, Drivers |
| Trip completion (started trips that finish) | >= 98 % | Trips |
| Mobile money payment success | >= 95 % first try, 100 % settled within 24 h | Finance |
| Crash-free sessions (rider and driver apps) | >= 99.5 % | Store consoles / Sentry |
| API p95 latency | < 150 ms | Grafana API panel |
| 5xx rate | < 0.5 % | Grafana API panel |
| Ledger verify | OK every day (`nexa verify-ledger`) | Daily review |
| Fare disputes unresolved > 24 h | 0 | Safety / support queue |
| Regulator submissions | 100 % accepted or retried within 1 h | Compliance > Regulator |

## 7. Daily review

**When:** 09:00 local, 15 minutes, every pilot day.
**Who:** pilot lead (chair), ops lead, backend on-call, mobile dev, support lead; finance on Mondays and Thursdays.

| Minute | Item | Look at |
|---|---|---|
| 0-3 | Yesterday's numbers vs targets | Dashboard home, Trips, Finance |
| 3-6 | Money and compliance | `verify-ledger` result, Finance (pay-ins, payouts), Compliance > Regulator |
| 6-9 | System health | Grafana: API p95 and 5xx, dispatch match rate and time to accept, worker job failures, Postgres and Redis |
| 9-12 | Driver and rider feedback | Live map coverage gaps, Drivers (offline, low acceptance), Safety (SOS, disputes) |
| 12-15 | Triage and owners | New issues from the tracker |

### Triage

| Class | Meaning | Goes to | Response |
|---|---|---|---|
| Bug | Code is wrong | Backend or mobile issue tracker | Fix and deploy through UAT; JS-only app fixes via EAS Update |
| Ops | People or process | Ops lead | Same day (driver coaching, support script, document follow-up) |
| Config | Data is wrong | Ops lead with admin access | Same day in admin (tariffs, area boundary, ride type, branding), logged in Audit |

Every issue gets a class, an owner and a due date before the meeting ends.

## 8. Timeline

| Week | Goal | Key actions | Exit |
|---|---|---|---|
| Week 0: prep | Everything ready | Production deployed and smoke-tested; area and tariffs live; 20 drivers recruited; onboarding sessions (5 drivers a day); internal store builds out; rider invite list sent; Grafana board and alerts set; support line staffed | 20 drivers fully onboarded, every checklist item green |
| Week 1: closed pilot | Prove the loop | Staff riders only for days 1-2, then all 50 riders; daily review; fix-forward on bugs | No money integrity issue; metrics trending to target |
| Week 2: widen | Prove it under load | Riders may invite 1 more person each (cap 100); extend hours to 06:00-22:00 if support allows; payout run tested twice | Go/no-go meeting on the last day |

## 9. Go / no-go criteria

**Go to launch** when all hold on the 7-day rolling numbers:

- [ ] Every target in section 6 met, or a written, accepted exception from the pilot lead.
- [ ] Zero unresolved money integrity issues; ledger verify OK every day of week 2.
- [ ] Two payout runs completed with every driver paid the right amount.
- [ ] No open severity 1 or 2 bugs.
- [ ] Launch checklist (`launch-checklist.md`) complete apart from items marked post-launch.

### Pause or roll back triggers

| Trigger | Action |
|---|---|
| Any money integrity issue (ledger verify fails, double charge, payout to wrong number, missing pay-in) | Pause bookings at once: deactivate the area in admin **Areas** (or turn off the related feature flag), then investigate |
| API p95 > 500 ms sustained for 15 min | Roll back to the previous image if a deploy is the cause; otherwise pause bookings |
| 5xx rate > 2 % for 10 min | Same as above |
| Crash-free sessions < 98 % on a new app build | Roll back the EAS Update or pull the build from internal testing |
| Regulator submissions failing for > 1 h | Pause bookings in that country's areas until submissions recover (where the regulator requires live reporting) |
| SOS not reaching staff | Pause bookings until fixed |

## 10. How to roll back

| What broke | Rollback |
|---|---|
| Backend release | Redeploy the previous image tag with the `deploy` workflow (Actions > deploy > Run workflow, environment `production`, reviewer approval). Migrations are forward-only: if a migration must be undone, ship a new corrective migration. |
| Database data | Follow the restore drill in `infra/README.md` (Backups section): point-in-time restore of managed Postgres to a new instance, verify with `verify-ledger`, then switch the connection string. |
| Mobile app, JS-only change | `eas update:rollback` (or republish the previous update) on the affected channel. |
| Mobile app, native change | Pull the build from internal testing / halt staged rollout; re-promote the previous build. |
| Config change | Revert in admin; the change and its author are in Audit. |
| Bookings must stop now | Deactivate the area in admin **Areas**; drivers and riders see "not available in this area"; trips already running finish. |

After any rollback: note it in the pilot tracker, raise a bug, and cover it at the next daily review.
