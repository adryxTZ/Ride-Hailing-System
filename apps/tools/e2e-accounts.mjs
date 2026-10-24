// Finance › Accounts against the real API: every account is listed with a
// balance that matches the ledger; recording a VAT payment lowers what we owe
// by exactly that amount; the same Idempotency-Key again posts nothing; more
// than is owed is refused; the entries and CSV show the payment; the chain verifies.
// Run: API=http://localhost:8093/v1 DB=nexa_accounts node tools/e2e-accounts.mjs
import { execFileSync } from "node:child_process";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const api = process.env.API ?? "http://localhost:8080/v1";
const db = process.env.DB ?? "nexa";
const PAY = "125.50";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const pass = (m) => log(`PASS ${m}`);
const fail = (m) => { throw new Error(m); };
const psql = (sql) => execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-d", db, "-qtAc", sql]).toString().trim();
async function call(method, path, token, body, expect, headers = {}) {
  const r = await fetch(api + path, { method, headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}`, ...headers },
    body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 300)}`);
  return { status: r.status, body: text && r.headers.get("content-type")?.includes("json") ? JSON.parse(text) : text };
}
const login = await fetch(`${api}/auth/staff/login`, { method: "POST", headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD }) });
const staff = (await login.json()).access_token;
const year = `from=${new Date().getFullYear()}-01-01`;
const list = async () => (await call("GET", `/admin/finance/accounts?${year}`, staff)).body;

// 1) every account listed; balances match the ledger
const before = await list();
const brandCode = process.env.BRAND ?? "nexa"; // the brand the API serves (NEXA_BRAND)
for (const kind of ["PLATFORM_REVENUE", "VAT_PAYABLE", "LEVY_PAYABLE", "DRIVER_WALLET", "RIDER_DUES", "RIDER_REFUNDS", "GATEWAY_CLEARING", "PLATFORM_BANK"]) {
  if (!before.accounts.some((a) => a.key === kind)) fail(`no ${kind} line`);
}
for (const line of before.accounts) {
  const [kind, payee] = line.key.split(":");
  const sum = psql(`select coalesce(sum(e.amount),0) from ledger_entries e join ledger_accounts a on a.id=e.account_id
    join brands b on b.id=a.brand_id where b.code='${brandCode}' and a.kind='${kind}' and a.currency='${before.currency}'
    ${payee ? `and a.owner_id='${payee}'` : ""}`);
  const credit = ["PLATFORM_REVENUE", "VAT_PAYABLE", "LEVY_PAYABLE", "CHARGES_PAYABLE", "RIDER_REFUNDS"].includes(kind);
  if (Math.abs((credit ? -Number(sum) : Number(sum)) - Number(line.balance)) > 0.001) fail(`${line.key}: line ${line.balance}, ledger ${sum}`);
}
const vat = before.accounts.find((a) => a.key === "VAT_PAYABLE");
pass(`balances listed: ${before.accounts.length} accounts in ${before.currency}; VAT we owe ${vat.balance}, fares ${before.fares} over ${before.trips} trips`);
if (Number(vat.balance) < Number(PAY)) fail(`VAT owed ${vat.balance} is below the test payment`);

// 2) record a VAT payment → VAT owed drops by it
const key = `e2e-accounts-${Date.now()}`;
const body = { amount: PAY, paid_on: new Date().toISOString().slice(0, 10), method: "BANK_TRANSFER", reference: "E2E-TRA-1", reason: "e2e: monthly VAT" };
const headers = { "Idempotency-Key": key };
const first = await call("POST", "/admin/finance/accounts/VAT_PAYABLE/payments", staff, body, 201, headers);
const afterPay = (await list()).accounts.find((a) => a.key === "VAT_PAYABLE");
const dropped = Math.round((Number(vat.balance) - Number(afterPay.balance)) * 100) / 100;
if (dropped !== Number(PAY)) fail(`VAT dropped by ${dropped}, paid ${PAY}`);
pass(`VAT payment ${PAY} recorded (${first.body.id.slice(0, 8)}): VAT owed ${vat.balance} → ${afterPay.balance}`);

// 3) the same key again → no second posting
const again = await call("POST", "/admin/finance/accounts/VAT_PAYABLE/payments", staff, body, 200, headers);
const postings = psql(`select count(*) from ledger_transactions where kind='finance.settlement' and reference='${first.body.id}'`);
const afterRepeat = (await list()).accounts.find((a) => a.key === "VAT_PAYABLE");
if (!again.body.replayed || again.body.id !== first.body.id || postings !== "1" || afterRepeat.balance !== afterPay.balance) {
  fail(`repeat: replayed=${again.body.replayed} postings=${postings} balance ${afterRepeat.balance}`);
}
pass("same Idempotency-Key again: 200 replayed, one posting, VAT unchanged");

// 4) refusals
const over = await call("POST", "/admin/finance/accounts/VAT_PAYABLE/payments", staff,
  { ...body, amount: String(Number(afterPay.balance) + 1) }, 409, { "Idempotency-Key": `${key}-over` });
const noKey = await call("POST", "/admin/finance/accounts/VAT_PAYABLE/payments", staff, body, 400);
const revenue = await call("POST", "/admin/finance/accounts/PLATFORM_REVENUE/payments", staff, body, 409, { "Idempotency-Key": `${key}-rev` });
pass(`refused: overpay → ${over.body.error.code} (owed ${over.body.error.details?.owed}); no key → ${noKey.body.error.code}; revenue → ${revenue.body.error.code}`);

// 5) entries and CSV show the payment; the bank paid it
const entries = (await call("GET", `/admin/finance/accounts/VAT_PAYABLE/entries?${year}&limit=5`, staff)).body;
const top = entries.entries[0];
if (top.kind !== "finance.settlement" || Number(top.amount) !== -Number(PAY) || top.running !== afterPay.balance) fail(`top entry ${JSON.stringify(top)}`);
const csv = (await call("GET", `/admin/finance/accounts/VAT_PAYABLE/export?${year}`, staff)).body;
const lines = csv.trim().split("\n");
if (!lines[0].startsWith("date,trip,kind") || !lines.at(-1).includes("finance.settlement")) fail(`csv ${lines[0]} … ${lines.at(-1)}`);
const bank = (await list()).accounts.find((a) => a.key === "PLATFORM_BANK");
pass(`entries: newest is the payment (−${PAY}, balance after ${top.running}, ${entries.total} in period); CSV ${lines.length - 1} rows; bank ${bank.balance}`);

// 6) the chain verifies
const verify = (await call("POST", "/admin/finance/verify", staff)).body;
if (!verify.ok) fail(`ledger chain broken at ${verify.broken_at}`);
pass(`POST /admin/finance/verify ok (${verify.checked} entries)`);
log("ACCOUNTS E2E PASSED");
