// Driver cash-outs against the real API (fake gateway, NEXA_FAKE_GATEWAY_DELAY=2s):
// the driver earns on mobile-money rides, asks to cash out, Finance approves, the
// gateway pays and only then the wallet moves; a second request is rejected with
// a reason; a third is cancelled by the driver; the rules refuse bad amounts.
// Run: node tools/e2e-payouts.mjs  (API on :8080)
import { execFileSync } from "node:child_process";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = "http://localhost:8080/v1";
const psql = (sql) =>
  execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql], { stdio: ["ignore", "pipe", "pipe"] }).toString().trim();
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(m); };
async function call(method, path, token, body, expect, headers = {}) {
  const r = await fetch(api + path, {
    method,
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...headers },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 300)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token;
};
const key = () => ({ "Idempotency-Key": `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}` });
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const wallet = () => call("GET", "/driver/wallet", dt);

// Cash-outs are limited per day; age the test driver's earlier ones so this run is not refused.
psql("update driver_payouts set created_at = created_at - interval '2 days' where driver_id = (select id from users where phone='+255715123702')");
// Close anything left open by an earlier run.
for (const p of (await call("GET", "/driver/payouts", dt)).data.filter((x) => x.status === "REQUESTED")) {
  await call("POST", `/driver/payouts/${p.id}/cancel`, dt, {});
}

// Earn on mobile-money rides until there is something to cash out.
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
async function mobileMoneyRide() {
  const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: { lat: -6.7764, lng: 39.229 }, dropoff: { lat: -6.7724, lng: 39.2181 } });
  let t = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "MOBILE_MONEY" }, 201);
  for (let i = 0; i < 30; i++) {
    const o = await call("GET", "/driver/offer", dt);
    if (o?.trip_id === t.id || o?.offer?.trip_id === t.id) break;
    await sleep(500);
  }
  t = await call("POST", `/driver/offers/${t.id}/accept`, dt);
  for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT"]) {
    t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
  }
  for (let i = 0; i < 40 && (await call("GET", `/trips/${t.id}`, rt)).status !== "COMPLETED"; i++) await sleep(500);
}
let w = await wallet();
for (let i = 0; i < 4 && Number(w.payouts.available_payout) < 12000; i++) {
  await mobileMoneyRide();
  w = await wallet();
  log(`after a mobile-money ride the platform owes the driver ${w.payouts.available_payout}`);
}
await call("PUT", "/driver/status", dt, { online: false });
const available = Number(w.payouts.available_payout), min = Number(w.payouts.payout_min);
if (!w.payouts.payouts_enabled || available < 2 * min) fail(`not enough to test: ${JSON.stringify(w.payouts)}`);

// The rules.
const below = await call("POST", "/driver/payouts", dt, { amount: String(min - 1) }, 400, key());
const above = await call("POST", "/driver/payouts", dt, { amount: String(available + 1) }, 400, key());
log(`below the minimum → ${below.error.code}; above the balance → ${above.error.code}`);

// 1) approved and paid
const before = Number(w.balance);
let p = await call("POST", "/driver/payouts", dt, { amount: String(min) }, 201, key());
log(`cash-out of ${p.amount} requested → ${p.status}`);
const second = await call("POST", "/driver/payouts", dt, { amount: String(min) }, 409, key());
log(`a second one while it is open → ${second.error.code}`);
const listed = (await call("GET", "/admin/finance/payouts?status=REQUESTED", staff)).data.find((x) => x.id === p.id);
if (!listed) fail("Finance does not see the request");
p = await call("POST", `/admin/finance/payouts/${p.id}/approve`, staff, { reason: "e2e: checked" });
log(`Finance approved → ${p.status}`);
if (Number((await wallet()).balance) !== before) fail("the wallet moved before the gateway paid");
for (let i = 0; i < 60 && (p = await call("GET", `/driver/payouts/${p.id}`, dt)).status === "PENDING"; i++) await sleep(1000);
const after = Number((await wallet()).balance);
log(`gateway: ${p.status}; driver balance ${before} → ${after}`);
if (p.status !== "PAID" || before - after !== min) fail("paid payout did not move the wallet by its amount");

// 2) rejected with a reason
p = await call("POST", "/driver/payouts", dt, { amount: String(min) }, 201, key());
const noReason = await call("POST", `/admin/finance/payouts/${p.id}/reject`, staff, { reason: "" }, 400);
p = await call("POST", `/admin/finance/payouts/${p.id}/reject`, staff, { reason: "e2e: number check" });
log(`rejected without a reason → ${noReason.error.code}; with one → ${p.status} (${p.decision_reason})`);

// 3) cancelled by the driver
p = await call("POST", "/driver/payouts", dt, { amount: String(min) }, 201, key());
p = await call("POST", `/driver/payouts/${p.id}/cancel`, dt, {});
const late = await call("POST", `/admin/finance/payouts/${p.id}/approve`, staff, { reason: "too late" }, 409);
log(`driver cancelled → ${p.status}; approving it now → ${late.error.code}`);

// Leave the shared test driver near zero, so the debt tests that follow start from a clean wallet.
w = await wallet();
if (Number(w.payouts.available_payout) >= min) {
  let rest = await call("POST", "/driver/payouts", dt, { amount: w.payouts.available_payout.split(".")[0] }, 201, key());
  await call("POST", `/admin/finance/payouts/${rest.id}/approve`, staff, { reason: "e2e: drain the test wallet" });
  for (let i = 0; i < 60 && (rest = await call("GET", `/driver/payouts/${rest.id}`, dt)).status === "PENDING"; i++) await sleep(1000);
  log(`cashed out the rest (${rest.amount}) → ${rest.status}; balance now ${(await wallet()).balance}`);
}

const audit = await call("GET", `/admin/audit?action=payout.&limit=5`, staff);
log(`audit: ${audit.data.slice(0, 2).map((a) => a.action).join(", ")}`);
const v = await call("POST", "/admin/finance/verify", staff);
if (!v.ok) fail("ledger chain broken");
log(`ledger chain ok (${v.checked} entries)`);
log("PAYOUTS E2E PASSED");
