// Pricing history against the real API (spec 2026-10-23-pricing-profiles-geofences.md, S3):
// 1) a region profile (commission 17%, a 400 levy) is published, switched on and applied; T1 is noted;
// 2) it is switched off; "as of" T1 still names it with v1 and the levy, "as of" now names the default;
// 3) the simulator with that (now inactive) profile previewed shows it winning and the levy in the
//    lines and the split, and saves no quote; without the preview the default applies;
// 4) the timeline lists the assignment with who and why; the records CSV downloads; a long range is refused.
// Run: node tools/e2e-pricing-history.mjs  (API on :8080; Docker Postgres for checks)
import { execFileSync } from "node:child_process";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(m); };
const psql = (sql) =>
  execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql], { stdio: ["ignore", "pipe", "pipe"] }).toString().trim();
const LEVY = 400;

async function call(method, path, token, body, expect, raw) {
  const r = await fetch(api + path, {
    method,
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 400)}`);
  return raw ? text : text ? JSON.parse(text) : null;
}

const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const areaId = psql(`select id from areas where name='Dar es Salaam' limit 1`);
const tag = String(Date.now()).slice(-6);

// Free the region (earlier runs leave a profile applied).
for (const p of (await call("GET", "/admin/pricing/profiles?country=TZ", staff)).data)
  for (const s of p.scopes.filter((x) => x.kind === "AREA" && x.id === areaId))
    await call("DELETE", `/admin/pricing/profiles/${p.id}/assignments/${s.assignment_id}?reason=e2e%20cleanup`, staff, undefined, 204);

// 1) profile on the region
const payee = await call("POST", "/admin/pricing/payees", staff, { country: "TZ", name: `E2E history payee ${tag}`, kind: "MUNICIPALITY", reason: "e2e" }, 201);
const prof = await call("POST", "/admin/pricing/profiles", staff, { country: "TZ", name: `E2E history ${tag}`, reason: "e2e" }, 201);
await call("POST", `/admin/pricing/profiles/${prof.id}/versions`, staff, {
  terms: { commission_rate: "0.17" }, reason: "e2e v1",
  charges: [{ code: "E2E_HLEVY", label: { en: "City levy", sw: "Ushuru wa jiji" }, category: "LEVY", amount_type: "FLAT", amount: String(LEVY),
    recipient: "THIRD_PARTY", payee_id: payee.id, applies_when: "EITHER", on_fixed_routes: true }],
}, 201);
await call("POST", `/admin/pricing/profiles/${prof.id}/status`, staff, { status: "ACTIVE", reason: "e2e" });
await call("POST", `/admin/pricing/profiles/${prof.id}/assignments`, staff, { scope_kind: "AREA", scope_id: areaId, reason: "e2e history" }, 201);
await sleep(1500);
const t1 = new Date().toISOString();
await sleep(1500);

// 2) switch off, then ask "as of"
await call("POST", `/admin/pricing/profiles/${prof.id}/status`, staff, { status: "INACTIVE", reason: "e2e off" });
const q = `country=TZ&lat=${PICKUP.lat}&lng=${PICKUP.lng}`;
const then = await call("GET", `/admin/pricing/as-of?${q}&at=${encodeURIComponent(t1)}`, staff);
log(`as of T1: ${then.profile.name} v${then.profile.version} on ${then.scope.kind}/${then.scope.name}, charges ${then.charges.map((c) => c.code)}`);
if (then.profile.id !== prof.id || then.profile.version !== 1 || !then.charges.some((c) => c.code === "E2E_HLEVY")) fail("as-of T1 must name the region profile v1 with its levy");
const now = await call("GET", `/admin/pricing/as-of?${q}&at=${encodeURIComponent(new Date().toISOString())}`, staff);
if (now.profile.id === prof.id) fail("as-of now must not name the switched-off profile");
log(`as of now: ${now.profile.name} (${now.scope.kind})`);
const future = await call("GET", `/admin/pricing/as-of?${q}&at=${encodeURIComponent(new Date(Date.now() + 3600e3).toISOString())}`, staff, undefined, 400);
log(`as of the future → ${future.error.code}`);

// 3) simulator: preview the inactive profile; nothing saved
await sleep(5500); // pricing caches check versions every 5 s
const quotesBefore = psql("select count(*) from quotes");
const plain = await call("POST", "/admin/pricing/simulate", staff, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
if (plain.resolution.profile.id === prof.id) fail("without a preview the inactive profile must not apply");
const sim = await call("POST", "/admin/pricing/simulate", staff, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF, preview_profile_id: prof.id });
const opt = sim.options.find((o) => o.available);
const split = sim.splits[opt.vehicle_class];
log(`simulate with preview: ${sim.resolution.profile.name}; ${opt.vehicle_class} ${opt.fare}, third party ${split.third_party}, commission ${split.commission}`);
if (sim.resolution.profile.id !== prof.id || !opt.lines.some((l) => l.meta?.charge === "E2E_HLEVY") || Number(split.third_party) !== LEVY) fail("the preview must apply the profile and its levy");
if (psql("select count(*) from quotes") !== quotesBefore) fail("the simulator saved a quote");
const stored = await call("GET", `/admin/pricing/profiles/${prof.id}`, staff);
if (stored.status !== "INACTIVE") fail("a preview changed the stored status");
log("simulator saved nothing; the profile is still switched off");
const far = await call("POST", "/admin/pricing/simulate", staff, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF, at: new Date(Date.now() + 9 * 86400e3).toISOString() }, 400);
log(`simulate 9 days ahead → ${far.error.code}`);
if (far.error.code !== "SIMULATE_AT_OUT_OF_RANGE") fail("expected SIMULATE_AT_OUT_OF_RANGE");

// 4) timeline and CSV
const from = new Date(Date.now() - 86400e3).toISOString(), to = new Date().toISOString();
const tl = await call("GET", `/admin/pricing/history?country=TZ&scope_kind=AREA&scope_id=${areaId}&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, staff);
const row = tl.assignments.find((a) => a.profile_id === prof.id);
if (!row || row.reason !== "e2e history" || !tl.events.some((e) => e.profile_id === prof.id && e.kind === "DEACTIVATED")) fail("the timeline must show the assignment and the switch-off");
log(`timeline: ${tl.assignments.length} assignments on the region, ${tl.events.length} events`);
const csv = await call("GET", `/admin/pricing/records.csv?country=TZ&from=${encodeURIComponent(new Date(Date.now() - 7 * 86400e3).toISOString())}&to=${encodeURIComponent(to)}`, staff, undefined, 200, true);
const lines = csv.trim().split("\n");
if (!lines[0].startsWith("trip_id,finalized_at")) fail("CSV header missing");
log(`CSV: ${lines.length - 1} records`);
const long = await call("GET", `/admin/pricing/records.csv?country=TZ&from=2026-01-01T00:00:00Z&to=${encodeURIComponent(to)}`, staff, undefined, 400);
log(`CSV over 31 days → ${long.error.code}`);

// Clean up: take the profile off the region (it stays in history).
const full = await call("GET", `/admin/pricing/profiles/${prof.id}`, staff);
for (const s of full.scopes) await call("DELETE", `/admin/pricing/profiles/${prof.id}/assignments/${s.assignment_id}?reason=e2e%20cleanup`, staff, undefined, 204);
log("PRICING HISTORY E2E PASSED");
