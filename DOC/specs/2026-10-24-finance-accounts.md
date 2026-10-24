# Spec: finance accounts (every party paid from fares) and recording payments

Written 2026-10-24. Owner's ask: "I hope we have accounts with ledger for all entities that are supposed to be paid as a percentage of the fare; it should show how much they owe or how much we have earned." Builds on the hash-chained ledger (`internal/money`, migration 00014), charge payees and `CHARGES_PAYABLE` (00035), refunds and `RIDER_REFUNDS` (00030, 00040/00041). Migration **00046** (00042–00045 are taken by features built in parallel).

## 1. Goal
Finance staff open **Money › Accounts** and see one line per party, in the selected country's currency (never summed across currencies):

| Line | Ledger kind | Position | Payable |
|---|---|---|---|
| Platform revenue | `PLATFORM_REVENUE` | Earned | no |
| VAT | `VAT_PAYABLE` | We owe (tax authority) | yes |
| Regulator levy (label from the country, e.g. "LATRA levy") | `LEVY_PAYABLE` | We owe | yes |
| Each charge payee (airport, municipality, parking operator …) | `CHARGES_PAYABLE:<payee>` | We owe | yes |
| Drivers (all wallets) | `DRIVER_WALLET` | Owed to us / we owe, both shown | no (cash-outs do it); links to driver balances |
| Rider dues | `RIDER_DUES` | Owed to us | no |
| Rider refunds | `RIDER_REFUNDS` | We owe | yes |
| Payment gateway | `GATEWAY_CLEARING` | Held at gateway | no |
| Platform bank | `PLATFORM_BANK` (new) | Bank: negative = net paid out | no |

For each: balance now; in the period (default this month, `from`/`to` dates in the country's timezone) what was **added** and what was **paid or cleared**; for fare-share parties (revenue, VAT, levy, payees) the **fare base** (fares of the period's completed trips the party took a share of) and the **effective rate** (share / base). Words on screen: "We owe", "Owed to us", "Earned".

A detail page per account (`/finance/accounts/<key>`) lists entries (date, trip code link, description, holder for per-person accounts, amount, running balance) with period filter, paging (50) and **CSV export** (oldest first; text cells beginning `= + - @ \t \r` are prefixed with `'`; amounts are server-formatted decimals and stay numeric).

**Record payment** on payable accounts: amount, paid-on date (not in the future), method (bank transfer, mobile money, cash, cheque, other), reference, reason. One balanced posting `finance.settlement` (reference = settlement id): debit the payable(s), credit `PLATFORM_BANK`. Overpaying is refused (`PAYMENT_MORE_THAN_OWED`, `details.owed`). `Idempotency-Key` is required; the same key again returns the first payment with 200 and `replayed: true`; the key used for a different payment is `IDEMPOTENCY_KEY_REUSED`.

Rider refunds can be paid three ways: oldest first (whole refunds only; `PAYMENT_AMOUNT_MISMATCH` with `below`/`above` otherwise), chosen refunds (`refund_ids`, amount must be their sum), or one rider's balance (`rider_id`; covers late fare money that has no refund row; refunds the amount fully covers, oldest first, are marked paid). Covered `trip_refunds` rows become `PAID` with `paid_ref` = the settlement id. Every rider's ledger balance must cover their part.

## 2. Contract (backend/api/openapi.yaml)
- `GET /admin/finance/accounts?country&from&to` → `FinanceAccounts` (finance.view)
- `GET /admin/finance/accounts/{key}/entries?country&from&to&limit&offset` → `FinanceAccountEntries` (finance.view)
- `GET /admin/finance/accounts/{key}/export?country&from&to` → `text/csv` (finance.view)
- `GET /admin/finance/rider-refunds/owed?country` → `OwedRiderRefunds` (finance.view)
- `POST /admin/finance/accounts/{key}/payments?country` + `Idempotency-Key` → 201/200 `FinancePayment` (**finance.settle**, audited `finance.payment`)

Scope: all routes sit behind `scopeGuard` like the other finance routes (currency = the scoped country's). Recording a payment also needs whole-country access (`allowedScope(country, "")`), as payout decisions do, and a payee must be one of that country's payees. Errors (en + sw): `ACCOUNT_NOT_FOUND`, `ACCOUNT_NOT_SETTLEABLE`, `PAYMENT_MORE_THAN_OWED`, `PAYMENT_AMOUNT_MISMATCH`, `REFUND_NOT_OWED`; reused `IDEMPOTENCY_KEY_REQUIRED`, `IDEMPOTENCY_KEY_REUSED`, `VALIDATION_FAILED` (`details.field`).

## 3. Data (00046_finance_accounts.sql)
- `account_kind` += `PLATFORM_BANK` (stays on Down: enum values cannot be dropped and entries are append-only).
- `finance_settlements` (kind, payee, rider, currency, amount, paid_on, method, reference, reason, idempotency_key, created_by); unique `(brand_id, idempotency_key)`; index `(brand_id, account_kind, currency, created_at DESC)`.
- Indexes: `ledger_transactions (brand_id, kind, created_at)` (period's trip.completed postings), `ledger_entries (transaction_id)` (their legs), `trip_refunds (brand_id, currency, created_at) WHERE status='OWED'` (oldest-first refunds).
- Permission `finance.settle`, granted to the Finance role (Super Admin has `*`).

## 4. Design
- `internal/accounts` (new module): `keys.go` (key ↔ kind/payee, each kind's position/sign/settleable/fare-share), `service.go` (list, entries, export, owed), `settle.go` (validation, overpay, posting, audit), `allocate.go` (pure rider-refund allocation), `csv.go`, `store.go` (one statement each for list and entries), `store_settle.go`.
- `internal/money/settlement.go`: `PlatformBank`, `LockTx` (the ledger's brand advisory lock, taken before the owed balance is read so it cannot move before the posting), `PostSettlementTx` (balanced; idempotent per settlement id; chain kept by `PostTx`).
- A payment is one DB transaction: lock → idempotency lookup → owed check → insert settlement → mark refunds paid → post → audit.
- Admin: `features/finance/accounts` (AccountsScreen, AccountDetail, PaymentDialog, PeriodPicker, hooks, labels); nav Money › Accounts.

## 5. Performance (definition of done)
One query per page: the list is one statement (CTEs: per-account balances and period debits/credits, the period's trip fares, fare shares, totals); entries is one statement (window running balance over the account, then the page, then joins to transaction, refund and trip by primary key). Export reuses the entries statement (cap 50,000 rows). `EXPLAIN (ANALYZE, BUFFERS)` on `nexa_accounts` (dev copy: 2,130 entries, 837 transactions, 328 trips), brand `nexa`, TZS, 1–24 Oct:

- **List**: 5.1 ms execution, 2.4 ms planning. `ledger_transactions_kind_time_idx` bitmap scan for the period's 180 trip.completed rows; accounts by `ledger_accounts_brand_id_kind_owner_id_currency_key`; ledger_entries is seq-scanned (whole table is 63 buffers; balances need every entry of the brand's accounts anyway).
- **Entries, VAT (180 entries)**: 1.3 ms. `ledger_entries_account_idx` bitmap scan; transactions, refunds and trips by primary key per page row (50 loops, memoized).
- **Entries, drivers (251 entries, holder names)**: 1.9 ms; hash joins at this size.

Known scaling point: balances are sums over all entries of an account (same as the existing Summary). When a brand passes a few million entries, add a per-account balance snapshot (closing balance per day) and sum only entries after it.

## 6. Tests
- Go (no DB): keys round trip and refusals; only payables settleable; allocation (oldest first whole refunds, partial refused, overpay refused, chosen must match and be owed, one rider's balance marks covered refunds); CSV formula escaping and numeric amounts; payment validation (key, amount scale, method, reference, reason, date, refund targets).
- Go (DB, `NEXA_TEST_DATABASE_URL`, fresh brand): balances equal the sum of entries and the newest running balance for every line; fare base and VAT rate; a VAT payment is balanced, the bank moves by it, replay with the same key posts nothing, the key for another amount is refused, overpay refused, payee paid in full, revenue not payable, chain verifies; rider refunds oldest first, partial refused, paid refund refused, chosen refund paid, no OWED rows left, chain verifies.
- e2e `apps/tools/e2e-accounts.mjs` and screenshots `apps/tools/shot-accounts.mjs`.

## 7. Factory log
- Migration 00046 up, down, up on `nexa_accounts`: OK.
- `gofmt -l` clean; `go vet ./...` clean; `golangci-lint run ./...`: 0 issues; `make brand-check`: clean.
- `go test ./... -count=1` with `NEXA_TEST_DATABASE_URL=…/nexa_accounts`: all packages ok (accounts 12+1 tests).
- `redocly lint api/openapi.yaml`: valid (54 warnings, none on the new operations).
- Both TS clients regenerated (`pnpm -s gen` in frontend/packages/api-client and apps/packages/api-client); admin `tsc --noEmit` 0 errors; `biome check admin/src` clean.
- `API=http://localhost:8093/v1 DB=nexa_accounts node tools/e2e-accounts.mjs`:
  ```
  PASS balances listed: 13 accounts in TZS; VAT we owe 17721.6, fares 1161362.63 over 180 trips
  PASS VAT payment 125.50 recorded (06544f86): VAT owed 17721.6 → 17596.1
  PASS same Idempotency-Key again: 200 replayed, one posting, VAT unchanged
  PASS refused: overpay → PAYMENT_MORE_THAN_OWED (owed 17596.10); no key → IDEMPOTENCY_KEY_REQUIRED; revenue → ACCOUNT_NOT_SETTLEABLE
  PASS entries: newest is the payment (−125.50, balance after 17596.1, 182 in period); CSV 182 rows; bank -251
  PASS POST /admin/finance/verify ok (899 entries)
  ACCOUNTS E2E PASSED
  ```
  (It also checks every line's balance against a direct ledger sum.)
- Screenshots read and fixed: "Record payment" wrapped in the list's action column (now nowrap); the ledger memo now says "bank transfer" instead of the method code.

## 8. Not done / follow-ups
- Sweeps from the gateway to the bank are not posted yet, so `PLATFORM_BANK` only shows money paid out (negative). A "Record bank deposit" from `GATEWAY_CLEARING` would complete the cash picture.
- Drivers are settled by cash-outs and debt pay-ins, not from this page.
- The list table uses the shared `Table` (600 px inner scroll); with many payees the lower lines scroll inside the panel.
