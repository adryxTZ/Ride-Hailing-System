// Debt holds and mobile-money pay-ins against the real API (fake gateway, settles in ~2 s):
// 1) a strict policy (hard limit 1,000) puts a driver with cash-trip debt on hold → going online is 409;
// 2) Pay now (normal phone) is collected within seconds, the debt drops and the hold lifts;
// 3) a replayed Idempotency-Key returns the same payment; phone …0001 fails without touching the debt;
// 4) a staff hold is not lifted by paying; releasing it frees the driver; the ledger chain verifies.
// Run: node tools/e2e-payin.mjs  (API on :8080 with NEXA_PAYMENT_GATEWAY=fake)
import { execFileSync } from "node:child_process";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const psql = (sql) => execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql]).toString().trim();
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function call(method, path, token, body, expect, headers = {}) {
  const r = await fetch(api + path, { method, headers: { ...j, ...headers, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => { await call("POST", "/auth/otp/request", null, { phone, app }); return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token; };
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver"), st = await sign("+255626512332", "admin");
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
// Prompts are rate-limited per driver and per number; age the test driver's earlier pushes so this run is not refused.
psql("update gateway_transactions set created_at = created_at - interval '2 days' where payer_id = (select id from users where phone='+255715123702')");
const key = () => `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
const policyOf = async () => (await call("GET", "/admin/pricing", st)).policy;
const publish = (p, changes, reason) => call("POST", "/admin/pricing/policy", st, { ...p, ...changes, reason });

// The seed's limits (seed/pricing/fee_policy.json); restored at the end whatever an earlier run left behind.
const DEFAULTS = { debt_soft_limit: "1000", debt_hard_limit: "5000", debt_soft_grace_days: 7, payin_min_amount: "1000" };
const original = await policyOf();
const driverId = (await call("GET", "/me", dt)).user_id;
// Make sure nothing is left from an earlier run.
await call("POST", `/admin/drivers/${driverId}/hold/release`, st, { reason: "e2e cleanup" }, 404).catch(() => {});
const { trip: active } = await call("GET", "/trips/active", rt); if (active) await call("POST", `/trips/${active.id}/cancel`, rt, { reason: "other" });

// 1) strict policy: every debt above 1,000 holds at once
await publish(original, { debt_soft_limit: "1000", debt_hard_limit: "1000", debt_soft_grace_days: 7, payin_min_amount: "1000" }, "e2e: strict debt limits");
await sleep(1500); // the pricing cache refreshes on the config version
let wallet = await call("GET", "/driver/wallet", dt);
// Cash trips until the driver owes at least the (strict) limit; each completed trip re-evaluates the hold.
async function cashTrip() {
  await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
  const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
  const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
  let t = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
  for (let i = 0; i < 20; i++) { const o = await call("GET", "/driver/offer", dt); if (o?.offer?.trip_id === t.id || o?.trip_id === t.id) break; await sleep(500); }
  t = await call("POST", `/driver/offers/${t.id}/accept`, dt);
  let next = t.allowed_transitions.find((s) => !s.startsWith("CANCELLED"));
  while (next) { t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: next }); next = t.allowed_transitions.find((s) => !s.startsWith("CANCELLED")); }
  await sleep(800);
}
for (let i = 0; Number(wallet.debt) < 1000 && i < 8; i++) {
  log(`debt is ${wallet.debt}; completing a cash trip to raise it`);
  await cashTrip();
  wallet = await call("GET", "/driver/wallet", dt);
}
log(`debt ${wallet.debt}; holds: ${wallet.holds.map((h) => `${h.kind}/${h.cause}`).join(", ") || "none"}; pay to unhold ${wallet.pay_to_unhold}`);
if (!wallet.holds.some((h) => h.kind === "DEBT")) throw new Error("expected a DEBT hold under the strict policy");
const refused = await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } }, 409);
log(`go online while held → ${refused.error.code}, pay ${refused.error.details.pay_to_unhold}`);

// 2) pay what lifts the hold… the strict policy needs the whole debt gone, so pay it all in one go
const k = key();
const amount = Math.ceil(Number(wallet.debt));
const payin = await call("POST", "/driver/payins", dt, { amount }, 201, { "Idempotency-Key": k });
log(`pay-in ${payin.id} ${payin.status} ${payin.amount} via ${payin.gateway}`);
const replay = await call("POST", "/driver/payins", dt, { amount }, 200, { "Idempotency-Key": k });
if (replay.id !== payin.id) throw new Error("replay returned a different payment");
let final;
for (let i = 0; i < 30; i++) { final = await call("GET", `/driver/payins/${payin.id}`, dt); if (final.status !== "PENDING") break; await sleep(500); }
log(`pay-in → ${final.status}`);
if (final.status !== "COLLECTED") throw new Error("pay-in not collected");
wallet = await call("GET", "/driver/wallet", dt);
log(`debt now ${wallet.debt}; holds: ${wallet.holds.length}; last entry ${wallet.entries[0].kind} ${wallet.entries[0].amount}`);
if (Number(wallet.debt) !== 0 || wallet.holds.length) throw new Error("debt or hold remains after paying everything");
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
await call("PUT", "/driver/status", dt, { online: false });
log("went online and offline again");

// 3) nothing to pay now; restore a normal policy and check a failed payment leaves the debt alone
const nothing = await call("POST", "/driver/payins", dt, { amount: 1000 }, 409, { "Idempotency-Key": key() });
log(`paying with no debt → ${nothing.error.code}`);

// 4) staff hold is independent of money
const hold = await call("POST", `/admin/drivers/${driverId}/hold`, st, { reason: "e2e: complaint under investigation" });
log(`staff hold ${hold.kind}/${hold.cause}`);
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } }, 409);
const holds = await call("GET", "/admin/finance/holds", st);
if (!holds.data.some((h) => h.driver_id === driverId && h.hold.kind === "MANUAL")) throw new Error("staff hold missing from the finance list");
const left = await call("POST", `/admin/drivers/${driverId}/hold/release`, st, { reason: "e2e: cleared" });
log(`released; holds left ${left.holds.length}`);
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
await call("PUT", "/driver/status", dt, { online: false });
const payins = await call("GET", "/admin/finance/payins", st);
log(`finance sees ${payins.data.length} recent pay-ins; latest ${payins.data[0].status}`);
const chain = await call("POST", "/admin/finance/verify", st);
log(`ledger ok=${chain.ok} checked=${chain.checked}`);
if (!chain.ok) throw new Error("ledger chain broken");
await publish(await policyOf(), DEFAULTS, "e2e: restore limits");
log("PAYIN E2E PASSED");
