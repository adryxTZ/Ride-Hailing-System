// Fraud v1 against the real API (spec 2026-10-23-compliance-trust.md, S3):
// 1) the rider and the driver sign in from the same app install (X-Install-Id);
// 2) the driver goes online reporting a mocked location → a HIGH GPS_SPOOF flag;
// 3) the rider books a ride naming the driver's phone as the passenger; the driver
//    completes it → a SELF_BOOKING flag on that trip with both signals;
// 4) staff dismiss the GPS flag with a note; a second review is refused; the
//    overview counts open flags.
// Run: node tools/e2e-fraud.mjs  (API on :8080 with Redis and workers)
import { randomUUID } from "node:crypto";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(m); };
const RIDER = "+255626512332", DRIVER = "+255715123702";
const install = randomUUID();

async function call(method, path, token, body, expect, headers = {}) {
  const r = await fetch(api + path, {
    method,
    headers: { "Content-Type": "application/json", ...headers, ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 400)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" }, undefined, { "X-Install-Id": install })).access_token;
};
const waitFor = async (what, fn) => {
  for (let i = 0; i < 40; i++) {
    const v = await fn();
    if (v) return v;
    await sleep(500);
  }
  fail(`timed out waiting for ${what}`);
};

const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const rt = await sign(RIDER, "rider"), dt = await sign(DRIVER, "driver");
const driverId = JSON.parse(Buffer.from(dt.split(".")[1], "base64url")).sub;
log(`rider and driver signed in from install ${install.slice(0, 8)}`);

// 2) mocked location
const active = await call("GET", "/trips/active", rt);
if (active.trip) await call("POST", `/trips/${active.trip.id}/cancel`, rt, { reason: "other" }).catch(() => {});
await call("PUT", "/driver/status", dt, { online: false });
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 }, mocked: true });
const gps = await waitFor("the GPS flag", async () =>
  (await call("GET", `/admin/fraud/flags?status=OPEN&rule=GPS_SPOOF&subject_id=${driverId}`, staff)).data[0]);
log(`GPS flag: ${gps.severity}, mocked=${gps.evidence.mocked}, hits ${gps.hits}`);
if (gps.severity !== "HIGH" || !gps.evidence.mocked) fail("a mocked location must raise a HIGH flag");

// 3) self-booking ride
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const trip = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH",
  contacts: { pickup: { name: "Passenger", phone: "0715123702" } } }, 201);
await waitFor("the offer", async () => { const o = await call("GET", "/driver/offer", dt); return o?.trip_id === trip.id || o?.offer?.trip_id === trip.id; });
let t = await call("POST", `/driver/offers/${trip.id}/accept`, dt);
for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT", "COMPLETED"])
  t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
log(`trip ${t.code} completed`);
const self = await waitFor("the self-booking flag", async () =>
  (await call("GET", `/admin/fraud/flags?rule=SELF_BOOKING&subject_id=${driverId}`, staff)).data.find((f) => f.trip_id === t.id));
const codes = self.evidence.signals.map((s) => s.code).sort().join(",");
log(`self-booking flag: ${self.severity}, signals ${codes}`);
if (codes !== "PASSENGER_IS_DRIVER,SHARED_INSTALL" || self.severity !== "HIGH") fail("expected both signals, HIGH");

// 4) review
const overview = await call("GET", "/admin/overview?country=TZ", staff);
if (!(overview.fraud_open >= 2)) fail(`overview fraud_open is ${overview.fraud_open}`);
const noNote = await call("POST", `/admin/fraud/flags/${gps.id}/review`, staff, { decision: "DISMISS", note: "" }, 400);
const done = await call("POST", `/admin/fraud/flags/${gps.id}/review`, staff, { decision: "DISMISS", note: "e2e: test device" });
const again = await call("POST", `/admin/fraud/flags/${gps.id}/review`, staff, { decision: "CONFIRM", note: "again" }, 409);
log(`review: no note → ${noNote.error.code}; dismissed by ${done.reviewed_by}; again → ${again.error.code}`);
await call("POST", `/admin/fraud/flags/${self.id}/review`, staff, { decision: "DISMISS", note: "e2e: test accounts" });
await call("PUT", "/driver/status", dt, { online: false });
log("FRAUD E2E PASSED");
