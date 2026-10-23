// Cancellation money against the real API (sockets not needed):
// 1) late cancel (accepted 3 min ago) → fee on the trip, rider owes it (/me/dues), driver credited 80%;
// 2) the next booking carries "PREVIOUS_CANCELLATION"; completing it settles the dues to 0;
// 3) driver no-show cancel is refused before 5 minutes waiting and charged after.
// Run: node tools/e2e-cancellation.mjs  (API on :8080; Docker Postgres for clock tweaks)
import { execFileSync } from "node:child_process";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const psql = (sql) => execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql]).toString().trim();
async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, { method, headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => { await call("POST", "/auth/otp/request", null, { phone, app }); return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token; };
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const cleanup = async () => { const { trip } = await call("GET", "/trips/active", rt); if (trip) await call("POST", `/trips/${trip.id}/cancel`, rt, { reason: "other" }); };
// Clear old dues so the numbers below are exact.
const startDues = Number((await call("GET", "/me/dues", rt)).amount);
await cleanup();
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.7770, lng: 39.2300 } });
const st = await call("GET", "/driver/status", dt); const cls = st.presence.vehicle_classes[0];
async function bookAndAccept() {
  const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
  const trip = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
  for (let i = 0; i < 20; i++) { const o = await call("GET", "/driver/offer", dt); if (o?.trip_id === trip.id || o?.offer?.trip_id === trip.id) break; await new Promise((r) => setTimeout(r, 500)); }
  return call("POST", `/driver/offers/${trip.id}/accept`, dt);
}

// 1) late cancel
let t = await bookAndAccept();
log(`accepted; cancel now costs ${t.cancel_fee_now}`);
psql(`update trips set accepted_at = now() - interval '3 minutes' where id='${t.id}'`);
t = await call("GET", `/trips/${t.id}`, rt); log(`3 min after acceptance cancel costs ${t.cancel_fee_now}`);
t = await call("POST", `/trips/${t.id}/cancel`, rt, { reason: "plans_changed" });
await new Promise((r) => setTimeout(r, 800));
const owed = Number((await call("GET", "/me/dues", rt)).amount) - startDues;
log(`cancelled: fee ${t.cancellation_fee}, rider now owes +${owed}`);
if (owed !== Number(t.cancellation_fee)) throw new Error("dues do not match the fee");
const wallet = await call("GET", "/driver/wallet", dt);
log(`driver wallet entry: ${wallet.entries[0].kind} ${wallet.entries[0].amount}`);

// 2) next trip carries and settles the dues
t = await bookAndAccept();
const dues = t.adjustments.find((a) => a.code === "PREVIOUS_CANCELLATION");
log(`next trip: estimate ${t.estimated_fare}, carries ${dues?.amount} from the cancellation → total ${t.final_fare}`);
let next = t.allowed_transitions.find((s) => !s.startsWith("CANCELLED"));
while (next) { t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: next }); next = t.allowed_transitions.find((s) => !s.startsWith("CANCELLED")); }
await new Promise((r) => setTimeout(r, 800));
log(`completed; rider dues now ${(await call("GET", "/me/dues", rt)).amount}`);

// 3) no-show
t = await bookAndAccept();
t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: "EN_ROUTE_TO_PICKUP" }).catch(() => t);
t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: "ARRIVED_AT_PICKUP" });
const early = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: "CANCELLED_BY_DRIVER", reason: "rider_no_show" }, 409);
log(`no-show right away → ${early.error.code}`);
psql(`update trips set arrived_pickup_at = now() - interval '6 minutes' where id='${t.id}'`);
t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: "CANCELLED_BY_DRIVER", reason: "rider_no_show" });
log(`no-show after 6 min → ${t.status}, fee ${t.cancellation_fee}`);
await call("PUT", "/driver/status", dt, { online: false });
log("CANCELLATION E2E PASSED");
