# Spec: rider mobile-money fares, driver payouts and receipts (Phase B, slice 2)

Written 2026-10-21 by the spec-writer agent (condensed; defaults in §9 apply until the owner answers). Builds on `2026-10-21-debt-holds-payins.md`. Fixes two existing gaps: non-cash bookings "complete" with no money moving (D12 #22), and `storeFees` failures are only logged.

## 1. Goal
A rider who books with **Mobile money** gets a USSD prompt for the final fare when the driver ends the ride; on approval the trip completes with a digital settlement (driver earnings credited, platform share to revenue, VAT/levy payable, money in `GATEWAY_CLEARING`). If the prompt fails or times out the rider retries (max 3) or pays cash, and the driver confirms cash as today. A trip never reaches COMPLETED without a terminal payment state. A driver with a positive balance **cashes out** to their account phone; Finance approves each payout unless the fee policy says automatic; the ledger moves only on gateway PAID. Every completed trip gets **one immutable, gapless-numbered receipt** (JSON for both apps, PDF in EN/SW via a 15-minute signed link).

## 2. Scope
**In:** `RIDE_FARE` purpose on `gateway_transactions`; `trips.payment_status` DUE|PENDING|FAILED|COLLECTED|CASH_CONFIRMED with a CHECK that COMPLETED needs COLLECTED/CASH_CONFIRMED; final fare and split fixed at AWAITING_PAYMENT; automatic prompt, rider retry (other number allowed), rider "pay cash", driver cash confirm after failure; settlement in the same tx as the gateway status change via `payments.Refresh`; late money after close → `RIDER_REFUNDS` (audited, visible to Finance); CARD/WALLET refused; `payment.Payouter` port (fake + Selcom disbursement); `payouts` module (min, one open, account phone only, REQUESTED→PENDING→PAID|FAILED, staff approve/reject or auto, `payout.paid` posted with PAID); `receipts` module (issued in the COMPLETED tx, gapless per brand/year, immutable trigger, JSON + signed PDF from a dependency-free writer `kernel/pdf`); apps and admin screens.
**Out:** card/wallet fares; in-app refunds; gateway fee accounting and bank sweep; emailed receipts; receipts for older trips; fiscal (TRA VFD/EFD) receipts; dispute flow; payouts to other numbers; app translation layer (deferred slice).

## 3. Contract (summary)
Schemas `TripPayment`, `FareCollection`, `Receipt {number, lines, total, taxes, payment, issuer…}`, `ReceiptLine`, `Payout`, `AdminPayout`. Trips: `POST /trips` refuses CARD/WALLET (`PAYMENT_METHOD_UNAVAILABLE`); `Trip.payment`; `POST /trips/{id}/payment/prompt` (Idempotency-Key; `PAYMENT_NOT_DUE`, `FARE_PROMPT_PENDING`, `FARE_PROMPT_LIMIT`, `PAYIN_RATE_LIMITED`, `GATEWAY_UNAVAILABLE`); `POST /trips/{id}/payment/cash`; driver COMPLETED only when DUE/FAILED (409 `FARE_PROMPT_PENDING` while open). Receipts: `GET /trips/{id}/receipt` (ETag, immutable cache), `GET /trips/{id}/receipt/link?lang`, public `GET /receipts/{id}/pdf?lang&exp&sig` (`RECEIPT_LINK_INVALID`), admin receipt link. Payouts: wallet gains `available_payout`, `payout_min`, `payouts_enabled`, `open_payout`; `POST /driver/payouts` (`PAYOUTS_UNAVAILABLE`, `PAYOUT_BELOW_MINIMUM`, `PAYOUT_ABOVE_BALANCE`, `NOTHING_TO_PAY_OUT`, `PAYOUT_IN_PROGRESS`, `PAYOUT_BLOCKED_BY_HOLD`, `PAYOUT_RATE_LIMITED`); `GET /driver/payouts`, `/{id}`, `/{id}/cancel`; admin `GET /admin/finance/payouts`, `approve`, `reject`; pay-ins list gains `purpose`; `FinanceSummary` adds `owed_to_riders`, `payouts_in_progress`, `payouts_paid_today`; policy adds `payout_min_amount`, `payout_auto_approve`; app config features `mobile_money_fares`, `payouts`.

## 4. Data
Migration (NO TRANSACTION): `account_kind` += `RIDER_REFUNDS`; `trips.payment_status`, `payment_ref` (backfill COMPLETED→CASH_CONFIRMED, AWAITING→DUE; CHECK); `gateway_transactions` purpose CHECK += RIDE_FARE, `trip_id`, `pushed_at`, trip index; `fee_policies.payout_min_amount 5000`, `payout_auto_approve false`; `driver_payouts` (one open per driver, pending/recent indexes); `receipt_counters (brand, year, last)`; `receipts` (trip unique, number unique per brand, jsonb lines/taxes/issuer/trip_info/driver_info, immutable trigger). Receipt number `<YYYY>-<6 digits>`. Ledger kinds: `trip.completed` digital legs (clearing +total; driver wallet −earnings; revenue; VAT; levy), `trip.dues_collected` digital, `fare.unapplied`, `payout.paid`. Env: `NEXA_SELCOM_PAYOUT_PIN` optional (payouts off without it); receipt link key derived from the JWT key dir; `NEXA_PUBLIC_API_URL` for links.

## 5. Behaviour (summary)
Payment rules pure in `trips/payment.go`; trips call `PaymentOpener` and `CompletedHook` ports (money stays out of trips). Entering AWAITING_PAYMENT fixes the fare/split (split failure aborts), opens a fare collection for MOBILE_MONEY (below gateway minimum → cash fallback), `fare_push` job sends the prompt, `Refresh` settles: COLLECTED → `CompletePaidTx` (COMPLETED, COLLECTED, events, receipt) + digital postings in one tx, then notify, hold evaluation, wallet frame; late success after close → `RIDER_REFUNDS`. Rider retry ≤ 3, per-phone cap 10/h. Payouts: `Create` (replay, availability, MANUAL hold blocks, min, ≤ balance, 3/day, one open; STAFF → REQUESTED, AUTO → PENDING + disburse job), `Approve` under a driver lock with balance re-check, `disburse` worker (re-check, reference first, never fail on an ambiguous reply), `Refresh` (PAID posts `payout.paid`; stuck after 30 min flagged), check + reconcile jobs. Receipts: `IssueTx` builds lines, asserts sum = total, snapshots issuer/driver, takes the gapless counter, `ON CONFLICT DO NOTHING`; signed PDF links (HMAC, 15 min, constant-time compare); deterministic A4 PDF from `kernel/pdf` (Helvetica, WinAnsi).

## 6. Apps and admin (summary)
Rider: payment method choice (Cash / Mobile money, persisted), `FarePaymentCard` (pending countdown, failed with retry / other number / cash), receipt card with "Download PDF". Driver: TripPanel waiting/failed/paid states for mobile-money fares, "Cash collected" confirm, `CashOutCard`, `PayoutSheet`, `PayoutStatus`, receipt row in history. Admin: Payouts card (approve/reject, stuck chip), pay-ins purpose filter, "Owed to riders" and "Payouts in progress" tiles, policy fields, receipt link on the trip page. EN/SW copy tables as in the full spec.

## 7. Tests (summary)
Go: money fare postings; trips payment rules and CHECK; payments fares (DB + fake: auto prompt, success completes with receipt and credit, failures, expiry, 4th prompt 429, cash paths, late success, 10-way race, webhook alone never completes, below-minimum fallback, CARD 400); receipts (lines sum, gapless 20 concurrent, immutability, participant check, ETag, signing, PDF structure and determinism); payouts (validation, staff and auto flows, balance drop before disburse, ambiguous error stays PENDING, stuck, 5 concurrent approves); fake and Selcom payout adapters; httpx routes/roles. E2E: `e2e-fares.mjs`, `e2e-payouts.mjs`, extended web and admin e2e.

## 8. Acceptance
- [ ] Migration applies/rolls back; completed trips backfilled CASH_CONFIRMED; CHECK holds.
- [ ] CARD/WALLET refused; a MOBILE_MONEY trip cannot complete without money or cash confirmation.
- [ ] AWAITING_PAYMENT fixes the fare; a mobile-money ride prompts automatically and completes on COLLECTED.
- [ ] One balanced digital `trip.completed` (+ dues) in the status tx; driver credited, no debt; chain verifies.
- [ ] Failure/timeout/outage → retry (≤ 3) or cash; driver cannot confirm cash while a prompt is open; late money owed to the rider, audited.
- [ ] Payouts: minimum, account phone, one open, staff approve/reject audited, auto mode, balance re-checked, never failed on ambiguity, posted only on PAID.
- [ ] One immutable gapless receipt per completed trip; lines sum to total; JSON in both apps; PDF EN/SW via signed link; tampered link refused.
- [ ] Apps show every state with eased motion; admin shows payouts, fare collections, tiles, policy fields, receipt.
- [ ] OpenAPI + clients; lint/tsc/brand grep; `doc.go` in payouts, receipts, kernel/pdf.
- [ ] p95 and queries recorded for receipt JSON, PDF, payout create, fare prompt, AWAITING_PAYMENT transition.

## 9. Owner questions (defaults in use)
1. Payout minimum 5,000 TZS, max 3/day. 2. Platform pays gateway fees (not booked yet). 3. Automatic payouts off (Finance approves). 4. Payouts only to the account phone. 5. Double payments owed to the rider, refunded by Finance outside the app. 6. Receipts are not TRA fiscal receipts; confirm whether fiscalisation (TIN/VRN) is needed before launch. 7. 3 prompts per trip, 3 minutes each. 8. Card fares wait for Selcom card credentials. 9. Selcom wallet cash-in access and PIN needed per environment.

## Factory log
- 2026-10-21: spec written; queued after operating areas.
- 2026-10-23: built. Deviations from this spec:
  - Payouts use Selcom's **Business API** (`/v1/transaction/process` and `/query`, RSA-signed, configured with `NEXA_SELCOM_PAYOUT_BASE_URL`, `_API_KEY` and `_KEY_FILE`), as the reference system does, not a wallet PIN. Payouts are off when these are unset, and drivers are told so.
  - Card and wallet fares are refused at booking (`PAYMENT_METHOD_UNAVAILABLE`).
  - The daily cap counts real cash-outs only; rejected and cancelled requests don't count.
  - Receipt PDF links are signed with a key derived from the JWT signing key.
  - New permission `payouts.approve`, added to the default Finance role.
  - Not yet recorded: p95 and queries per request for the new endpoints.
- 2026-10-23, review round 1 (security and code reviewers both asked for changes). Fixed:
  - A driver could cancel another driver's cash-out: Cancel is now scoped to the caller.
  - Selcom: only known refusal codes fail a payout; 5xx, 429 and unknown codes stay pending. A sweep re-checks payouts refused in the last 24 h and posts any the gateway paid after all (audited `payout.late_paid`).
  - Cash-trip debt and carried-over dues are posted inside the completing transaction. A failed fare split now aborts completion instead of being only logged.
  - A collected fare that cannot settle its trip (split or receipt mismatch) is owed back to the rider (audited, logged), and the trip is freed for cash instead of retrying forever. Transient database errors still retry.
  - Prompt retries with the same key return the original prompt, and a key from another trip is refused (`IDEMPOTENCY_KEY_REUSED`).
  - The automatic prompt runs in the background, so the driver's "end ride" does not wait on the gateway.
  - Approving or rejecting a cash-out, and staff reading receipts, now check the staff member's country or region scope.
  - Receipt links are built from `NEXA_PUBLIC_API_URL` only, except on localhost (`PUBLIC_URL_NOT_CONFIGURED` otherwise).
  - The open cash-out is subtracted from what the driver can cash out.
  - The cash-out list reads balances in one grouped query.
  - The receipt-building logic moved out of `cmd/` into `receipts/trip.go`, and receipt build failures are logged as errors.
  - Go tests were added for the fare path (completion and single posting under races, owed-back money, settlement failure, prompt cap and key replay), cross-driver cash-out access, the late-paid sweep, and Selcom unknown codes.
- Still open (low): receipt numbers serialise completions per brand per year on one counter row (accepted for gapless numbering; load-check at Phase E); the SQL status fragments are constants.

