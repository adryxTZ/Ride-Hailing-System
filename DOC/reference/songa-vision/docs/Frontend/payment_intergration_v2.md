# Songa × Selcom Payments — Frontend Integration Skill (v2)

Audience: the AI/developer working on **songa_driver_app** and **songa-rider-app** (Flutter)
and, where noted, **songa_admin**. This is the authoritative contract for the Selcom payment
features. Everything marked **[UAT-VERIFIED]** was proven against the live gateway with real
money on 2026-08-17 (merchant 70053934, real TZS charged and reconciled). Build the UI
exactly to these behaviors — the backend state machine is deliberately conservative and the
UI must mirror it honestly.

Base URL: the app's normal API host (prod `https://songa.kalen.co.tz`).
Auth: normal driver/rider JWT (`Authorization: Bearer <access>`, obtained via the existing
OTP login `POST /api/users/generate-otp/` → `POST /api/users/verify-otp/`).

---

## 0. The mental model (read this first)

Money moves in FOUR distinct flows. Know which one you're building for:

| # | Flow | Direction | Trigger | Status today |
|---|------|-----------|---------|--------------|
| 1 | **Driver debt pay-in (USSD push)** | driver → Songa | Driver taps "Lipa deni" | ✅ **LIVE, UAT-verified with real money** |
| 2 | Driver debt declare (manual) | driver → Songa (offline) | Driver declares, Finance verifies | ✅ live (fallback path) |
| 3 | Driver payout (disbursement) | Songa → driver | Admin dashboard or auto cron — **NEVER the driver** | ⛔ blocked: Selcom hasn't enabled payout APIs for our vendor (error 4035). Build read-only UI only |
| 4 | Rider card charging (charge-first) | rider → Songa | Automatic at trip request when all gates pass | 🟡 backend live behind switches; card registration needs a live Selcom round to finalize |

Key money-safety principle the UI must respect: **a payment is only "done" when the backend
says COMPLETED.** "Push sent", "popup shown", even "PIN entered" are NOT payment. The backend
confirms against Selcom (webhook + a 5-minute reconciliation poller) and flips the status;
the UI's job is to show honest intermediate states and never invent success.

---

## 1. Driver debt pay-in via USSD push  ✅ [UAT-VERIFIED]

The driver owes Songa the service charge from cash trips (`wallet.debt`). They pay it from
their own mobile-money wallet via a USSD popup.

### 1.1 Fire the push

```
POST /api/payments/settlements/
Authorization: Bearer <driver JWT>
{ "amount": "1000.00", "pay_now": true }
```

201 response:
```json
{
  "settlement_id": "70efab7c-…",
  "amount": "1000.00",
  "payment_method": "MOBILE_MONEY",
  "reference_number": "SNG-PAYIN-4C59875ADA7D481CBCFF",
  "status": "PENDING",
  "notes": "USSD push pay-in",
  "verified_at": null,
  "created_at": "…",
  "payin_status": "PROCESSING"
}
```

### 1.2 What happens on the phone [UAT-VERIFIED]

- The USSD popup lands on the driver's registered number within seconds.
- The flow asks the driver to **confirm/enter the AMOUNT (digits only, e.g. `1000`)** and
  then their **mobile-money wallet PIN** (M-Pesa/Mixx/Airtel PIN — the one they use to send
  money; NOT any Songa password). Surface this in the UI copy:
  *"Angalia simu yako. Weka kiasi (mfano 1000) kisha PIN yako ya Mixx/M-Pesa."*
- **The USSD session expires in roughly 90 seconds.** Tell the driver to have the phone in hand
  BEFORE tapping pay.
- **Minimum amount: TZS 1,000.** [UAT-VERIFIED: 200 and 500 were rejected by the operator with
  "umeingiza kiasi kisicho sahihi"; 1,000 succeeded.] Client-side: block pay_now below 1000
  with a clear message. (If wallet.debt < 1000, hide the USSD button and offer the manual
  declare path §2 — or let them pay nothing until debt accumulates.)
- The operator charges its own fee + VAT on top (driver's receipt showed Ada TZS 120 + VAT 18
  on a 1,000 payment) — that's between the driver and their wallet provider; Songa receives
  the face amount.

### 1.3 Status lifecycle (drive the UI off `status`)

```
PENDING  ──(driver pays; backend confirms with Selcom ≤5 min)──► COMPLETED
   │                                                             debt reduced,
   └──(push failed / expired / declined / never paid)──────────► REJECTED
```

- After firing: show "Inasubiri malipo…" and poll `GET /api/payments/settlements/` every
  ~10–20 s (or wait for the socket event, §5). Confirmation typically lands **within 5
  minutes of PIN entry** (the backend reconciler runs on a 5-minute cadence; the webhook can
  make it faster).
- `COMPLETED`: refresh wallet (debt is reduced), show success + the reference. The driver
  also receives a Songa SMS in Swahili.
- `REJECTED`: show "Malipo hayakukamilika — jaribu tena" with a retry button. **There is no
  auto-retry by design** — the driver re-initiates whenever they want. Each retry creates a
  brand-new settlement + reference.
- NEVER show a settlement as paid based on the popup appearing or the 201 response.

### 1.4 Errors (all JSON `{"error": "…"}`)

| HTTP | Message contains | UI behavior |
|------|------------------|-------------|
| 400 | "USSD pay-in is currently disabled" | Admin switch off — hide/disable the button, offer manual declare (§2) |
| 400 | "exceeds debt" | Client-validate amount ≤ wallet.debt first |
| 400 | "Invalid amount" | Client validation |
| 503 | "temporarily unavailable" | Gateway down — retry-later UI, don't hammer |
| 401 | token expired | normal refresh flow |

Note: the POST can take up to ~30–60 s (two Selcom calls). Use a generous client timeout
(120 s) and a blocking spinner; if the app gets a gateway 504 but a new settlement appears in
the GET list, treat it as fired (idempotent-safe: worst case the driver sees an extra PENDING
row that will auto-REJECT).

### 1.5 Settlement history

`GET /api/payments/settlements/` → array (latest first, ≤100) of the driver's settlements in
the same shape as §1.1 (without `payin_status`). Show status chips; PENDING items are
"awaiting confirmation", REJECTED items are retryable.

---

## 2. Driver debt declare (manual fallback) ✅

Same endpoint WITHOUT `pay_now`: the driver already paid Songa by other means and submits
the reference for Finance to verify manually.

```
POST /api/payments/settlements/
{ "amount": "1000.00", "payment_method": "MOBILE_MONEY",
  "reference_number": "MPESA-REF-XYZ", "notes": "optional" }
```
→ 201, `status: "PENDING"` → Finance verifies → `VERIFIED` → `COMPLETED` (debt reduced).
Statuses may also be `REJECTED`. UI: show "Inasubiri uthibitisho wa Songa" on pending items.
Show Songa's pay-to details (till/account) on this screen — obtain from the Songa team; not
served by the API.

---

## 3. Driver payouts (disbursements) — READ-ONLY UI for now ⛔

Payouts (Songa → driver wallet) are initiated by the **admin dashboard or a scheduled job,
NEVER in-app by the driver**. Do not build any "withdraw" button.

Currently **blocked at Selcom**: the payout APIs return `4035 API endpoint not enabled for
the vendor`. Selcom has been asked to enable Wallet Cashin for vendor 70053934. The backend
contract below is stable and will start returning data the moment that's resolved — build
the read side now:

- `GET /api/payments/disbursements/` → array of the driver's payouts:
  `{disbursement_id, amount, fee, status, initiated_by (ADMIN|AUTO|DRIVER),
  destination_fi_code, destination_account, destination_name, failure_reason, order_id,
  gateway_receipt, completed_at, created_at}`.
- Statuses: PENDING / PROCESSING / COMPLETED / FAILED / AMBIGUOUS. Wallet `balance` is
  **reserved (deducted) the moment a payout is created**; FAILED restores it. Show any
  non-terminal payout as "Malipo yanachakatwa: TZS X" next to the balance so the drop isn't
  alarming.
- Fees are platform-absorbed: the driver receives the full amount; `fee` is informational.
- **Prerequisite UI (build now):** payout destination = the driver's default MOBILE_MONEY
  PaymentMethod (`/api/payments/payment-methods/` CRUD). Provider must come from a FIXED
  picker: `M-Pesa | Airtel Money | Tigo Pesa (Mixx by Yas) | HaloPesa | T-Pesa`. A driver
  with a positive balance and no method on file should get a blocking prompt to add one.

---

## 4. Rider card payments (charge-first) 🟡 rider app

Riders can be charged on a **stored card** (Mastercard/Visa/Amex/UnionPay) instead of cash.
Model: the **estimated fare is charged BEFORE the trip starts**; any fare difference is
charged after the trip. If ANYTHING fails, the trip silently becomes CASH — a trip can never
end unpaid.

### 4.1 Three gates (ALL must pass, else = cash)
1. Global admin switch (server-side).
2. Rider activated card payments on their profile (§4.2).
3. Rider has a registered card token (§4.3).

### 4.2 Card/cash toggle — `/api/payments/cards/preference/`

- `GET` →
  ```json
  { "card_payments_activated": false, "has_card": false, "card": null,
    "feature_enabled": false, "effective_method": "CASH" }
  ```
  `effective_method` is the single source of truth for what the next trip will use — bind the
  payment-method UI to it. `card` carries `{last4, brand}` when present.
- `POST {"activate": true|false}` → toggles. Toggling ON returns 400
  "Card payments are currently disabled" (switch off) or "Add a card to your payment methods
  first" (no card). Toggling OFF always succeeds. Response = same shape as GET.

### 4.3 Card registration — `/api/payments/cards/register/` (POST)

→ 201 `{ "order_id": "SNG-CARDREG-…", "payment_page_url": "https://…" }`.
Open `payment_page_url` in a webview; the rider enters card details + 3-D Secure on
**Selcom's hosted page** (Songa never touches card numbers). On completion the backend
fetches and stores the token automatically; poll `GET /cards/preference/` until
`has_card: true`. A small registration charge (~TZS 1,000) validates the card.
Errors: 400 with the gate messages above. ⚠️ This flow still needs one live Selcom round to
confirm the hosted-page response fields — treat as beta; ship behind an app-side flag.

### 4.4 Trip request behavior (rider app)

`POST /api/trip/request/` (existing endpoint) with `payment_method: "CARD"`:
- Response now carries two additive fields:
  `"payment_method"` — what the trip ACTUALLY uses after the gates ran, and
  `"card_fallback_reason"` — non-null when CARD was requested but the trip fell back to CASH
  (switch off / not activated / no card / charge declined / gateway down).
- UI: if `card_fallback_reason` is set, toast *"Kadi haikupatikana — safari hii utalipa
  taslimu"* and show the trip as cash. Never block the trip because the card failed.
- Fare difference at trip end is charged automatically server-side; no rider action. The
  final receipt/invoice comes through the existing trip-completion payloads.

---

## 5. Real-time events (both apps)

Existing driver/rider socket (Netty gateway). New event on terminal payment states:

```json
{ "type": "payment_status_update",
  "reference": "SNG-PAYIN-…", "status": "COMPLETED" | "FAILED",
  "amount": "1000.00",
  "transaction_type": "COLLECTION" | "DISBURSEMENT",
  "driver_id": "DRV-…" }
```

- `COLLECTION` + COMPLETED → debt pay-in confirmed: refresh wallet + settlements, success toast.
- `COLLECTION` + FAILED → pay-in failed: refresh, offer retry.
- `DISBURSEMENT` → payout outcome (when payouts go live).
- Match list rows by `reference` == `reference_number`/`order_id`.
- The backend also sends a Swahili SMS on the same transitions — don't double-notify with an
  identical local notification if the socket event was just shown.

---

## 6. UAT facts your UX copy should encode [all verified live]

1. Popup arrives in seconds; **USSD session ≈ 90 s** — phone in hand first.
2. The prompt asks **amount then wallet PIN**; amount must be digits matching exactly
   (e.g. `1000`).
3. **Minimum TZS 1,000** per pay-in (operator-enforced).
4. Wallet PIN ≠ app password. Wrong PIN = operator cancels; wallets lock after ~3 wrong tries.
5. Confirmation can take **up to 5 minutes** after PIN (reconciler cadence) — design the
   waiting state for that, not for instant.
6. Unpaid/expired pushes self-resolve to REJECTED — stale PENDING rows disappear on their own;
   no cleanup UI needed.
7. Operator fee + VAT appear on the driver's own receipt, on top of the face amount.
8. OTP login: generate-otp uses field `email_or_phone`; a per-number rate limiter returns 429
   on rapid resends (show "subiri sekunde 60").

## 7. Quick test drive (against UAT server)

1. Login as a driver (OTP flow) → check `GET /api/users/driver/stats/` for wallet.debt.
2. `POST /api/payments/settlements/ {"amount":"1000.00","pay_now":true}` (debt must be
   ≥ 1000; ask backend team to seed).
3. Watch the phone → amount `1000` → wallet PIN.
4. Poll `GET /api/payments/settlements/` until COMPLETED (≤5 min), confirm debt dropped and
   the SMS + socket event arrived.
5. Error paths worth exercising: pay_now with amount > debt (400), toggling
   `/cards/preference/` with no card (400), settlements list rendering of REJECTED rows.
