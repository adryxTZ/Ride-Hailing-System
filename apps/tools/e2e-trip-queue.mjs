// End-to-end check of the trip queue (accept the next trip while finishing) against a running API:
// 1) the driver finishing trip A (at the drop-off) is offered trip B marked queued and accepts it;
//    B's rider sees "queued" with a pickup estimate;
// 2) B cannot start before A completes (NEXT_TRIP_WAITING); A's cash confirmation is not blocked;
// 3) completing A makes B the driver's active trip (and the socket says so);
// 4) going offline with a queued trip sends it back to matching;
// 5) with the country's trip queue switched off (staff, audited) a finishing driver gets no offers.
// Run: API=http://localhost:8094/v1 NEXA_DATABASE_URL=… NEXA_REDIS_URL=… STAFF_EMAIL=… STAFF_PASSWORD=… node tools/e2e-trip-queue.mjs
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const API = process.env.API ?? "http://localhost:8080/v1";
const WS = `${API.replace(/^http/, "ws")}/ws`;
const COUNTRY = process.env.COUNTRY ?? "TZ";
const RIDER = "+255626512332", DRIVER = "+255715123702";
const SERVICE = "taxi", CLASS = "standard";
const START = { lat: -6.777, lng: 39.23 };
const A_PICKUP = { lat: -6.7764, lng: 39.229 }, A_DROP = { lat: -6.7724, lng: 39.2181 };
const B_PICKUP = { lat: -6.773, lng: 39.219 }, B_DROP = { lat: -6.7764, lng: 39.229 };
const QUIET_MS = 4000; // a few dispatch ticks
const SETTINGS_PROPAGATION_MS = 6000; // just over the settings cache's version check (5 s)
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

// The approved vehicle decides what a driver serves: register the test driver for taxi / standard.
execFileSync(fileURLToPath(new URL("../../backend/bin/nexa", import.meta.url)),
  ["driver", "provision", DRIVER, "--service", SERVICE, "--class", CLASS, "--plate", "T123ABC", "--name", "Adrian Mvungi"],
  { cwd: fileURLToPath(new URL("../../backend/", import.meta.url)), stdio: "inherit",
    env: { ...process.env, NEXA_BRAND: process.env.NEXA_BRAND ?? "nexa", NEXA_LOG_LEVEL: "warn",
      NEXA_DATABASE_URL: process.env.NEXA_DATABASE_URL ?? "postgres://nexa:nexa@localhost:5433/nexa?sslmode=disable",
      NEXA_REDIS_URL: process.env.NEXA_REDIS_URL ?? "redis://localhost:6380/0" } });

const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const pass = (m) => console.log(`PASS ${m}`);
const fail = (m) => { throw new Error(m); };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function call(method, path, token, body, expect) {
  const r = await fetch(API + path, {
    method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  const data = text ? JSON.parse(text) : null;
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return data;
}
async function signIn(phone, app) {
  const req = await call("POST", "/auth/otp/request", null, { phone, app });
  const t = await call("POST", "/auth/otp/verify", null, { phone, code: req.dev_code ?? "0000", app });
  if (t.consent_required) {
    const doc = await call("GET", `/legal/consent?app=${app}`);
    await call("POST", "/me/consents", t.access_token, { app: app.toUpperCase(), version: doc.version });
  }
  return t.access_token;
}
function socket(token, name) {
  const frames = [];
  const ws = new WebSocket(WS);
  ws.onopen = () => ws.send(JSON.stringify({ type: "auth", token }));
  ws.onmessage = (e) => { const f = JSON.parse(e.data); frames.push(f); };
  const wait = async (pred, ms = 10000) => {
    const end = Date.now() + ms;
    while (Date.now() < end) { const f = frames.find(pred); if (f) { frames.splice(frames.indexOf(f), 1); return f; } await sleep(100); }
    fail(`${name}: timed out waiting for a frame`);
  };
  return { ws, frames, wait };
}
async function cancelActive(rt) {
  const active = await call("GET", "/trips/active", rt);
  if (active.trip) await call("POST", `/trips/${active.trip.id}/cancel`, rt, { reason: "other", note: "e2e cleanup" }).catch(() => {});
}
async function book(rt, pickup, dropoff) {
  await cancelActive(rt);
  const q = await call("POST", "/quotes", rt, { service: SERVICE, pickup, dropoff });
  return call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: CLASS, payment_method: "CASH",
    stops: [{ kind: "PICKUP", address: "E2E pickup" }, { kind: "DROPOFF", address: "E2E dropoff" }] });
}
const step = (dt, id, to) => call("POST", `/driver/trips/${id}/transitions`, dt, { to });
async function walk(dt, id, statuses) {
  let t;
  for (const to of statuses) t = await step(dt, id, to);
  return t;
}
const setQueue = (staff, enabled, reason) =>
  call("PATCH", `/admin/dispatch/settings/${COUNTRY}`, staff, { trip_queue_enabled: enabled, reason });
async function offerFor(driver, tripId) {
  return (await driver.wait((f) => f.type === "offer.new" && f.data.trip_id === tripId)).data;
}

const rt = await signIn(RIDER, "rider");
const rt2 = await signIn(`+25574${String(Math.floor(Math.random() * 1e7)).padStart(7, "0")}`, "rider");
const dt = await signIn(DRIVER, "driver");
const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
await setQueue(staff, true, "e2e: trip queue on");
await sleep(SETTINGS_PROPAGATION_MS); // a previous run may have left it off
// Leftovers from an interrupted run: the driver walks their trips to the end.
for (let stale = (await call("GET", "/driver/trips/active", dt)).trip; stale?.allowed_transitions.length;) {
  const next = stale.allowed_transitions.find((s) => !s.startsWith("CANCELLED")) ?? stale.allowed_transitions[0];
  stale = await step(dt, stale.id, next);
  if (!stale.allowed_transitions.length) stale = (await call("GET", "/driver/trips/active", dt)).trip;
}
await cancelActive(rt2);
await call("PUT", "/driver/status", dt, { online: true, location: START });
const driver = socket(dt, "driver"), rider2 = socket(rt2, "rider2");
await driver.wait((f) => f.type === "ready"); await rider2.wait((f) => f.type === "ready");

// 1) Trip A to the drop-off; the driver is finishing.
const a = await book(rt, A_PICKUP, A_DROP); log(`booked A ${a.code}`);
await offerFor(driver, a.id);
await call("POST", `/driver/offers/${a.id}/accept`, dt);
await walk(dt, a.id, ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF"]);
await call("POST", "/driver/location", dt, { location: A_DROP, heading: 90 });
const st = await call("GET", "/driver/status", dt);
if (!st.presence?.finishing || st.presence.state !== "ON_TRIP") fail(`driver at the drop-off should be finishing: ${JSON.stringify(st.presence)}`);
log("driver is finishing trip A");

const b = await book(rt2, B_PICKUP, B_DROP); log(`booked B ${b.code}`);
const offerB = await offerFor(driver, b.id);
if (!offerB.queued) fail("the offer to a finishing driver must be marked queued");
log(`queued offer: pickup ${offerB.pickup_distance_m} m from the drop-off, eta ${offerB.pickup_eta_s}s`);
const accB = await call("POST", `/driver/offers/${b.id}/accept`, dt);
if (accB.status !== "ACCEPTED" || !accB.queued || accB.allowed_transitions.includes("EN_ROUTE_TO_PICKUP")) fail(`accepted B: ${JSON.stringify(accB).slice(0, 200)}`);
const riderB = await call("GET", `/trips/${b.id}`, rt2);
if (!riderB.queued || !(riderB.queued_pickup_eta_s >= 60)) fail(`B's rider should see a finishing driver and an ETA: ${riderB.queued} ${riderB.queued_pickup_eta_s}`);
if ((await call("GET", "/driver/trips/active", dt)).trip?.id !== a.id) fail("trip A stays the driver's active trip");
const next = (await call("GET", "/driver/trips/next", dt)).next_trip;
if (next?.trip_id !== b.id) fail(`next trip banner: ${JSON.stringify(next)}`);
pass(`driver finishing trip A gets and accepts trip B (queued offer, rider ETA ${riderB.queued_pickup_eta_s}s, banner ${next.pickup_distance_m} m)`);

// 2) B cannot start before A completes; A's payment and cash confirmation go through.
const blocked = await call("POST", `/driver/trips/${b.id}/transitions`, dt, { to: "EN_ROUTE_TO_PICKUP" }, 409);
if (blocked.error?.code !== "NEXT_TRIP_WAITING") fail(`start before completion: ${JSON.stringify(blocked)}`);
pass("B cannot start before A completes (NEXT_TRIP_WAITING)");

// 3) Completing A makes B the active trip.
await walk(dt, a.id, ["AWAITING_PAYMENT", "COMPLETED"]);
const promoted = await driver.wait((f) => f.type === "trip.updated" && f.data.id === b.id && !f.data.queued);
const activeB = (await call("GET", "/driver/trips/active", dt)).trip;
const st2 = await call("GET", "/driver/status", dt);
if (activeB?.id !== b.id || !activeB.allowed_transitions.includes("EN_ROUTE_TO_PICKUP")) fail(`B should be active: ${JSON.stringify(activeB).slice(0, 200)}`);
if (st2.presence?.trip_id !== b.id || st2.presence.state !== "ON_TRIP" || st2.presence.finishing) fail(`presence after promotion: ${JSON.stringify(st2.presence)}`);
if ((await call("GET", `/trips/${b.id}`, rt2)).queued) fail("B's rider still sees a queued trip after A completed");
pass(`completing A makes B active (socket trip.updated ${promoted.data.status}, presence ON_TRIP on B)`);

// 4) Offline with a queued trip: B to the drop-off, a queued trip C, then offline.
await walk(dt, b.id, ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF"]);
await call("POST", "/driver/location", dt, { location: B_DROP, heading: 90 });
const c = await book(rt, A_PICKUP, A_DROP); log(`booked C ${c.code}`);
await offerFor(driver, c.id);
await call("POST", `/driver/offers/${c.id}/accept`, dt);
await call("PUT", "/driver/status", dt, { online: false });
const released = await call("GET", `/trips/${c.id}`, rt);
if (!["REQUESTED", "OFFERED"].includes(released.status) || released.driver_id) fail(`C after offline: ${released.status} ${released.driver_id}`);
pass(`going offline releases the queued trip back to matching (C is ${released.status})`);
await cancelActive(rt);

// 5) Trip queue off: a finishing driver gets no offers.
await call("PUT", "/driver/status", dt, { online: true, location: B_DROP });
await setQueue(staff, false, "e2e: trip queue off");
await sleep(SETTINGS_PROPAGATION_MS); // dispatchers re-check the settings version every 5 s
const d = await book(rt, A_PICKUP, A_DROP); log(`booked D ${d.code} with the queue off`);
await sleep(QUIET_MS);
if (driver.frames.some((f) => f.type === "offer.new" && f.data.trip_id === d.id)) fail("a finishing driver got an offer with the trip queue off");
const audit = await call("GET", "/admin/dispatch/settings", staff);
if (audit.data.find((x) => x.country === COUNTRY)?.trip_queue_enabled !== false) fail("settings did not save");
pass("toggle off → no queued offers to the finishing driver");
await cancelActive(rt);
await setQueue(staff, true, "e2e: restore the default");

await walk(dt, b.id, ["AWAITING_PAYMENT", "COMPLETED"]);
await call("PUT", "/driver/status", dt, { online: false });
driver.ws.close(); rider2.ws.close();
log("TRIP QUEUE E2E PASSED");
