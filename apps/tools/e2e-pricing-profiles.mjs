// Pricing profiles against the real API (spec 2026-10-23-pricing-profiles-geofences.md, S1):
// 1) a payee and a region profile (commission 15%) with a third-party levy (500) and a
//    driver parking charge (300) are published, switched on and assigned to the region;
// 2) a quote there shows both charges as labelled lines; a cash ride is booked, driven
//    and confirmed; the trip's pricing record is FINAL with that profile, version 1 and
//    the region; the ledger owes the payee 500 and the driver keeps 300 more;
// 3) publishing version 2 and switching the profile off changes nothing on that trip
//    (same record, same hash) and new quotes fall back to the country default;
// 4) the database refuses to change a final record; the default cannot be switched off.
// Run: node tools/e2e-pricing-profiles.mjs  (API on :8080; Docker Postgres for checks)
import { execFileSync } from "node:child_process";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const psql = (sql) =>
  execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql], { stdio: ["ignore", "pipe", "pipe"] }).toString().trim();
const fail = (m) => { throw new Error(m); };
const LEVY = 500, PARKING = 300, REGION_COMMISSION = "0.15";

async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, {
    method,
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 400)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token;
};

const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const areaId = psql(`select id from areas where name='Dar es Salaam' limit 1`);
if (!areaId) fail("no Dar es Salaam area");
const tag = String(Date.now()).slice(-6);

// Free the region: earlier runs leave their profile assigned.
const list = await call("GET", "/admin/pricing/profiles?country=TZ", staff);
for (const p of list.data) {
  for (const s of p.scopes.filter((x) => x.kind === "AREA" && x.id === areaId)) {
    await call("DELETE", `/admin/pricing/profiles/${p.id}/assignments/${s.assignment_id}?reason=e2e%20cleanup`, staff, undefined, 204);
  }
}

// 1) payee + region profile with charges
const payee = await call("POST", "/admin/pricing/payees", staff, { country: "TZ", name: `E2E authority ${tag}`, kind: "MUNICIPALITY", reason: "e2e" }, 201);
const prof = await call("POST", "/admin/pricing/profiles", staff, { country: "TZ", name: `E2E region ${tag}`, reason: "e2e" }, 201);
const charges = [
  { code: "E2E_LEVY", label: { en: "City levy", sw: "Ushuru wa jiji" }, category: "LEVY", amount_type: "FLAT", amount: String(LEVY),
    recipient: "THIRD_PARTY", payee_id: payee.id, applies_when: "EITHER", on_fixed_routes: true },
  { code: "E2E_PARKING", label: { en: "Parking", sw: "Maegesho" }, category: "PARKING", amount_type: "FLAT", amount: String(PARKING),
    recipient: "DRIVER", applies_when: "PICKUP", on_fixed_routes: true },
];
const v1 = await call("POST", `/admin/pricing/profiles/${prof.id}/versions`, staff, { terms: { commission_rate: REGION_COMMISSION }, charges, reason: "e2e v1" }, 201);
const vatOnRegion = await call("POST", `/admin/pricing/profiles/${prof.id}/versions`, staff, { terms: { vat_rate: "0.2" }, charges: [], reason: "e2e" }, 409);
log(`region profile cannot set VAT → ${vatOnRegion.error.code}`);
await call("POST", `/admin/pricing/profiles/${prof.id}/status`, staff, { status: "ACTIVE", reason: "e2e" });
await call("POST", `/admin/pricing/profiles/${prof.id}/assignments`, staff, { scope_kind: "AREA", scope_id: areaId, reason: "e2e" }, 201);
log(`profile ${prof.name} v${v1.version} active on Dar es Salaam with ${charges.length} charges`);

// 2) quote shows the charges; ride and confirm cash
await sleep(5500); // let the pricing cache see the change (version check every 5 s)
const active = await call("GET", "/trips/active", rt);
if (active.trip) await call("POST", `/trips/${active.trip.id}/cancel`, rt, { reason: "other" }).catch(() => {});
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const opt = q.options.find((o) => o.vehicle_class === cls);
const chargeLines = opt.lines.filter((l) => l.code === "CHARGE");
log(`quote ${cls}: ${opt.lines.map((l) => `${l.label?.en ?? l.code} ${l.amount}`).join(", ")} = ${opt.fare}`);
if (chargeLines.length !== 2 || chargeLines.reduce((n, l) => n + Number(l.amount), 0) !== LEVY + PARKING) fail("charges missing on the quote");
if (!chargeLines.every((l) => l.label?.en && l.label?.sw)) fail("charge lines need EN and SW labels");

const trip = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
for (let i = 0; i < 30; i++) {
  const o = await call("GET", "/driver/offer", dt);
  if (o?.trip_id === trip.id || o?.offer?.trip_id === trip.id) break;
  await sleep(500);
}
let t = await call("POST", `/driver/offers/${trip.id}/accept`, dt);
for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT"]) {
  t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
}
// Mid-ride, staff publish v2 (commission 20%, levy 900) and switch the profile off:
// the fare is split when the ride ends, and must still use what the trip was booked under.
await call("POST", `/admin/pricing/profiles/${prof.id}/versions`, staff, { terms: { commission_rate: "0.2" }, charges: [{ ...charges[0], amount: "900" }, charges[1]], reason: "e2e v2 mid-ride" }, 201);
await call("POST", `/admin/pricing/profiles/${prof.id}/status`, staff, { status: "INACTIVE", reason: "e2e mid-ride" });
await sleep(5500);
log("mid-ride: v2 published (20%, levy 900) and the profile switched off");
for (const to of ["ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT", "COMPLETED"]) {
  t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
}
const rec = await call("GET", `/admin/trips/${t.id}/pricing-record`, staff);
log(`record: ${rec.stage} ${rec.profile.name} v${rec.profile.version} ${rec.scope.kind}/${rec.scope.name}, default v${rec.default_profile?.version}, hash ${rec.record_hash?.slice(0, 12)}`);
if (rec.stage !== "FINAL" || rec.profile.id !== prof.id || rec.profile.version !== 1 || rec.scope.kind !== "AREA") fail("record does not show the region profile v1");
if (rec.charges.length !== 2) fail("record must list both charges");
const split = rec.final.breakdown;
if (split.profile_id !== prof.id || Number(split.third_party) !== LEVY) fail(`split did not use the pinned profile: ${JSON.stringify(split)}`);
if ("profile_id" in t.fees || "third_party" in t.fees || "platform_net" in t.fees) fail("apps must not receive the internal split");
const commission = Number(split.commission), rideFare = Number(split.ride_fare);
if (Math.abs(commission - rideFare * Number(REGION_COMMISSION)) > 1) fail(`commission ${commission} is not 15% of ${rideFare}`);
const owed = psql(`select coalesce(-sum(e.amount),0) from ledger_entries e join ledger_accounts a on a.id=e.account_id where a.kind='CHARGES_PAYABLE' and a.owner_id='${payee.id}'`);
log(`ledger: payee owed ${owed}; driver earnings ${t.fees.driver_earnings} include parking ${PARKING}`);
if (Number(owed) !== LEVY) fail("payee is not owed the levy");

// 3) changes after completion never touch the record either
const etag = rec.record_hash;
if (rec.seal_valid !== true) fail("the record's seal does not verify");
await call("POST", `/admin/pricing/profiles/${prof.id}/versions`, staff, { terms: { commission_rate: "0.25" }, charges: [], reason: "e2e v3" }, 201);
const again = await call("GET", `/admin/trips/${t.id}/pricing-record`, staff);
if (again.record_hash !== etag || again.profile.version !== 1 || again.seal_valid !== true) fail("a later version changed the trip's record");
log("v3 published after completion: the record and its seal are unchanged");
const q2 = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
if (q2.options.some((o) => o.lines.some((l) => l.code === "CHARGE"))) fail("an inactive profile still priced a quote");
log("new quotes use the country default again (no charges)");

// 4) guards
const changed = (() => { try { psql(`update trip_pricing_records set terms='{}' where trip_id='${t.id}'`); return true; } catch { return false; } })();
if (changed) fail("the database let a final record change");
log("database refuses to change a final pricing record");
const def = list.data.find((p) => p.is_default);
const off = await call("POST", `/admin/pricing/profiles/${def.id}/status`, staff, { status: "INACTIVE", reason: "e2e" }, 409);
log(`switching the default off → ${off.error.code}`);
const assign = await call("POST", `/admin/pricing/profiles/${def.id}/assignments`, staff, { scope_kind: "AREA", scope_id: areaId, reason: "e2e" }, 409);
log(`assigning the default to a region → ${assign.error.code}`);
const verify = await call("POST", "/admin/finance/verify", staff);
if (!verify.ok) fail("ledger chain broken");
await call("DELETE", `/admin/pricing/profiles/${prof.id}/assignments/${(await call("GET", `/admin/pricing/profiles/${prof.id}`, staff)).scopes[0].assignment_id}?reason=e2e%20cleanup`, staff, undefined, 204);
await call("PUT", "/driver/status", dt, { online: false });
log("PRICING PROFILES E2E PASSED");
