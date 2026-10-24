// Staff fare actions on a trip against the real API (dashboard leftovers):
// 1) during a ride staff set the price by hand (a reason is required); the trip shows
//    the new fare and a MANUAL_PRICE line; the cash split at the end uses it;
// 2) after completion the price is locked; staff refund part of it, recovered from
//    the driver; more than what is left is refused; the ledger owes the rider the refund
//    and the chain verifies.
// Run: node tools/e2e-trip-money.mjs  (API on :8080; Docker Postgres for checks)
import { execFileSync } from "node:child_process";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(m); };
const psql = (sql) => execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql]).toString().trim();
async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, { method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 300)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token;
};
const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const PRICE = 5000, REFUND = 1500;

const active = await call("GET", "/trips/active", rt);
if (active.trip) await call("POST", `/trips/${active.trip.id}/cancel`, rt, { reason: "other" }).catch(() => {});
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const trip = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
for (let i = 0; i < 30; i++) { const o = await call("GET", "/driver/offer", dt); if (o?.trip_id === trip.id || o?.offer?.trip_id === trip.id) break; await sleep(500); }
let t = await call("POST", `/driver/offers/${trip.id}/accept`, dt);
for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT"]) t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });

// 1) set price mid-ride
const noReason = await call("POST", `/admin/trips/${t.id}/price`, staff, { amount: String(PRICE), reason: "" }, 400);
const set = await call("POST", `/admin/trips/${t.id}/price`, staff, { amount: String(PRICE), reason: "e2e: agreed by phone" });
const line = set.adjustments.find((a) => a.code === "MANUAL_PRICE");
log(`price set: ${set.estimated_fare} → ${set.final_fare} (line ${line?.amount}); without a reason → ${noReason.error.code}`);
if (Number(set.final_fare) !== PRICE || !line) fail("the fare and its line must follow the new price");
for (const to of ["ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT", "COMPLETED"]) t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
const split = JSON.parse(psql(`select fees from trips where id='${t.id}'`));
log(`completed; split total ${split.total}, driver ${split.driver_earnings}`);
if (Number(split.total) !== PRICE) fail("the split must use the price staff set");

// 2) locked, then refund
const locked = await call("POST", `/admin/trips/${t.id}/price`, staff, { amount: "4000", reason: "too late" }, 409);
log(`price after completion → ${locked.error.code}`);
const before = await call("GET", `/admin/trips/${t.id}/refunds`, staff);
if (Number(before.refundable) !== PRICE) fail(`refundable ${before.refundable}`);
const key = `e2e-${t.id}`;
const refundOnce = () => fetch(`${api}/admin/trips/${t.id}/refunds`, { method: "POST",
  headers: { "Content-Type": "application/json", Authorization: `Bearer ${staff}`, "Idempotency-Key": key },
  body: JSON.stringify({ amount: String(REFUND), borne_by: "DRIVER", reason: "e2e: long detour" }) }).then((r) => r.json());
const after = await refundOnce();
const repeat = await refundOnce(); // a retried request with the same key refunds nothing more
if (repeat.refunds.length !== 1 || Number(repeat.refunded) !== REFUND) fail("a repeated idempotency key refunded twice");
log("the same Idempotency-Key again: still one refund");
log(`refunded ${REFUND} (driver bears it); left ${after.refundable}`);
const tooMuch = await call("POST", `/admin/trips/${t.id}/refunds`, staff, { amount: "4000", borne_by: "PLATFORM", reason: "e2e" }, 409);
log(`refund over what is left → ${tooMuch.error.code} (refundable ${tooMuch.error.details?.refundable})`);
const owed = psql(`select -sum(e.amount) from ledger_entries e join ledger_accounts a on a.id=e.account_id join ledger_transactions x on x.id=e.transaction_id
  where a.kind='RIDER_REFUNDS' and x.reference='${after.refunds[0].id}'`);
if (Number(owed) !== REFUND) fail(`the ledger owes the rider ${owed}`);
if (!(await call("POST", "/admin/finance/verify", staff)).ok) fail("ledger chain broken");
log(`ledger: rider owed ${owed}; chain verifies`);
await call("PUT", "/driver/status", dt, { online: false });
log("TRIP MONEY E2E PASSED");
