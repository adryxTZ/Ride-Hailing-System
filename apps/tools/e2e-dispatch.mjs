// End-to-end check of matching against a running API (no apps involved):
// driver goes online → rider books → driver's socket gets offer.new → decline → (re-offer is NOT
// sent to the same driver) → rider cancels; then a second booking → accept → the driver walks
// the trip to COMPLETED while the rider's socket receives every trip.updated.
// Run: API=http://localhost:8080/v1 node tools/e2e-dispatch.mjs
const API = process.env.API ?? "http://localhost:8080/v1";
const WS = API.replace(/^http/, "ws") + "/ws";
const RIDER = "+255626512332", DRIVER = "+255715123702";
// Any pickup-and-drop-off service works: SERVICE=delivery CLASS=boda node tools/e2e-dispatch.mjs
const SERVICE = process.env.SERVICE ?? "taxi", CLASS = process.env.CLASS ?? "standard";
// Site services (water, construction materials): SITE=1 ATTRS='{"water_type":"clean"}' — one location, no drop-off.
const SITE = process.env.SITE === "1", ATTRS = JSON.parse(process.env.ATTRS ?? "{}");
// The approved vehicle decides what a driver serves: register the test driver's vehicle for this service.
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
execFileSync(fileURLToPath(new URL("../../backend/bin/nexa", import.meta.url)),
  ["driver", "provision", "+255715123702", "--service", SERVICE, "--class", CLASS, "--plate", "T123ABC", "--name", "Adrian Mvungi"],
  { cwd: fileURLToPath(new URL("../../backend/", import.meta.url)), env: { ...process.env, NEXA_BRAND: "nexa", NEXA_DATABASE_URL: "postgres://nexa:nexa@localhost:5433/nexa?sslmode=disable", NEXA_REDIS_URL: "redis://localhost:6380/0", NEXA_LOG_LEVEL: "warn" }, stdio: "inherit" });
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function call(method, path, token, body) {
  const r = await fetch(API + path, {
    method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  const data = text ? JSON.parse(text) : null;
  if (!r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return data;
}
async function signIn(phone, app) {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, code: "0000", app })).access_token;
}
function socket(token, name) {
  const frames = [];
  const ws = new WebSocket(WS);
  ws.onopen = () => ws.send(JSON.stringify({ type: "auth", token }));
  ws.onmessage = (e) => { const f = JSON.parse(e.data); frames.push(f); log(`${name} ← ${f.type}${f.data?.status ? " " + f.data.status : ""}`); };
  const wait = async (pred, ms = 8000) => {
    const end = Date.now() + ms;
    while (Date.now() < end) { const f = frames.find(pred); if (f) { frames.splice(frames.indexOf(f), 1); return f; } await sleep(100); }
    throw new Error(`${name}: timed out waiting for frame`);
  };
  return { ws, frames, wait };
}
async function book(rt) {
  const active = await call("GET", "/trips/active", rt);
  if (active.trip) await call("POST", `/trips/${active.trip.id}/cancel`, rt, { reason: "other", note: "e2e cleanup" });
  const q = await call("POST", "/quotes", rt, { service: SERVICE, pickup: PICKUP, dropoff: SITE ? undefined : DROPOFF, attributes: ATTRS });
  return call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: CLASS, payment_method: "CASH", contacts: { dropoff: { name: "E2E Receiver", phone: "0712345678" } },
    stops: SITE ? [{ kind: "PICKUP", address: "E2E site" }] : [{ kind: "PICKUP", address: "E2E pickup" }, { kind: "DROPOFF", address: "E2E dropoff" }] });
}

const rt = await signIn(RIDER, "rider");
const dt = await signIn(DRIVER, "driver");
// Leftovers from an interrupted run: the driver walks their trip to the end.
let stale = (await call("GET", "/driver/trips/active", dt)).trip;
while (stale && stale.allowed_transitions.length) {
  const next = stale.allowed_transitions.find((s) => !s.startsWith("CANCELLED")) ?? stale.allowed_transitions[0];
  stale = await call("POST", `/driver/trips/${stale.id}/transitions`, dt, { to: next });
}
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.7770, lng: 39.2300 }, services: [SERVICE], vehicle_classes: [CLASS] });
const rider = socket(rt, "rider"), driver = socket(dt, "driver");
await rider.wait((f) => f.type === "ready"); await driver.wait((f) => f.type === "ready");
log("both sockets authenticated");

// 1) offer → decline → same driver not asked again
let trip = await book(rt); log(`booked ${trip.code}`);
let offer = await driver.wait((f) => f.type === "offer.new" && f.data.trip_id === trip.id);
log(`offer: ${offer.data.currency} ${offer.data.fare}, pickup ${offer.data.pickup_distance_m} m, eta ${offer.data.pickup_eta_s}s, ttl ${offer.data.ttl_s}s`);
await call("POST", `/driver/offers/${trip.id}/decline`, dt); log("driver declined");
await driver.wait((f) => f.type === "offer.closed");
await sleep(2500);
if (driver.frames.some((f) => f.type === "offer.new")) throw new Error("declined driver was offered the same trip again");
log("not re-offered to the driver who declined ✓");
await call("POST", `/trips/${trip.id}/cancel`, rt, { reason: "other", note: "e2e" }); log("rider cancelled trip 1");

// 2) offer → accept → full lifecycle
trip = await book(rt); log(`booked ${trip.code}`);
offer = await driver.wait((f) => f.type === "offer.new" && f.data.trip_id === trip.id);
const accepted = await call("POST", `/driver/offers/${trip.id}/accept`, dt);
log(`accepted → ${accepted.status}, driver ${accepted.driver?.name ?? accepted.driver?.phone}`);
await rider.wait((f) => f.type === "trip.updated" && f.data.status === "ACCEPTED");
try { await call("POST", `/driver/offers/${trip.id}/accept`, dt); throw new Error("double accept allowed"); } catch (e) { if (String(e).includes("double")) throw e; log("second accept rejected ✓"); }
await call("POST", "/driver/location", dt, { location: { lat: -6.7768, lng: 39.2296 }, heading: 200 });
const loc = await rider.wait((f) => f.type === "driver.location");
log(`rider got driver.location ${loc.data.location.lat},${loc.data.location.lng}`);
const active = await call("GET", "/trips/active", rt);
log(`rider GET active: driver_location ${active.trip.driver_location ? "present" : "MISSING"}`);
// Chat both ways while the trip is live.
await call("POST", `/trips/${trip.id}/messages`, rt, { body: "I'm at the gate", quick_reply: "at_pickup" });
const chat = await driver.wait((f) => f.type === "chat.message");
log(`driver got chat: "${chat.data.body}"`);
const page = await call("GET", `/trips/${trip.id}/messages`, dt);
log(`driver sees ${page.data.length} message(s), ${page.quick_replies.length} quick replies`);
await call("POST", `/trips/${trip.id}/messages`, dt, { body: "On my way" });
await rider.wait((f) => f.type === "chat.message" && f.data.sender_role === "DRIVER");
log("rider got the driver's reply");
// Follow the lifecycle the server allows (taxi, hauling and site services differ).
let next = accepted.allowed_transitions.find((s) => !s.startsWith("CANCELLED"));
while (next) {
  const to = next;
  const t = await call("POST", `/driver/trips/${trip.id}/transitions`, dt, { to });
  await rider.wait((f) => f.type === "trip.updated" && f.data.status === to);
  log(`→ ${t.status} (next: ${t.allowed_transitions.join(", ") || "—"})`);
  next = t.allowed_transitions.find((s) => !s.startsWith("CANCELLED"));
  // Pretend the driver waited 6 minutes at the pickup: the ride start adds the waiting fee.
  if (to === "ARRIVED_AT_PICKUP") {
    execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc",
      `update trips set arrived_pickup_at = now() - interval '6 minutes' where id='${trip.id}'`]);
  }
  if (to === "IN_TRANSIT") {
    const w = t.adjustments.find((a) => a.code === "WAITING");
    log(`waiting fee: ${w ? `${w.amount} for ${w.meta.minutes} min` : "none"}; fare ${t.estimated_fare} → ${t.final_fare}`);
    if (!w) throw new Error("no waiting fee after 6 minutes at the pickup");
  }
}
// Both sides rate once; chat is closed after the trip.
await call("POST", `/trips/${trip.id}/rating`, rt, { score: 5, aspects: ["friendly", "safe_driving"], comment: "Great" });
await call("POST", `/trips/${trip.id}/rating`, dt, { score: 4 });
try { await call("POST", `/trips/${trip.id}/rating`, rt, { score: 3 }); throw new Error("rated twice"); } catch (e) { if (String(e).includes("twice")) throw e; }
try { await call("POST", `/trips/${trip.id}/messages`, rt, { body: "late" }); throw new Error("chat open after trip"); } catch (e) { if (String(e).includes("after trip")) throw e; }
const ratings = await call("GET", `/trips/${trip.id}/ratings`, rt);
log(`ratings: ${ratings.data.map((r) => `${r.rater_role}=${r.score}`).join(", ")}; second rating and late chat rejected ✓`);
const st = await call("GET", "/driver/status", dt);
log(`driver presence after completion: ${st.presence?.state}`);
await call("PUT", "/driver/status", dt, { online: false });
rider.ws.close(); driver.ws.close();
log("DISPATCH E2E PASSED");
