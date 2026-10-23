// Geofences against the real API (spec 2026-10-23-pricing-profiles-geofences.md, S2):
// 1) a PLACE around the drop-off with a profile charging an entry fee on DROPOFF;
// 2) a ZONE on the quoted road (a small square at the route's middle) with a profile
//    charging a toll when the ROUTE_CROSSES it;
// 3) a quote shows both charges, labelled, and its pricing trace names the drop-off and crossed zone;
// 4) a self-intersecting shape and a tiny one are refused; an import dry run reports per feature;
// 5) two overlapping zones with active profiles at the same priority are refused (tie).
// Run: node tools/e2e-geofences.mjs  (API on :8080 with OSRM routing; Docker Postgres for clean-up)
import { execFileSync } from "node:child_process";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(m); };
const psql = (sql) =>
  execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql], { stdio: ["ignore", "pipe", "pipe"] }).toString().trim();
const ENTRY = 1000, TOLL = 700;
const SQUARE_DEG = 0.0012; // ~130 m half-size around the route's middle

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

/** Google/OSRM encoded polyline (precision 5) → [lat, lng] points. */
function decode(s) {
  const out = [];
  let i = 0, lat = 0, lng = 0;
  const next = () => {
    let shift = 0, result = 0, b;
    do { b = s.charCodeAt(i++) - 63; result |= (b & 0x1f) << shift; shift += 5; } while (b >= 0x20);
    return result & 1 ? ~(result >> 1) : result >> 1;
  };
  while (i < s.length) { lat += next(); lng += next(); out.push([lat / 1e5, lng / 1e5]); }
  return out;
}
const square = (lat, lng, d) => ({ type: "Polygon", coordinates: [[[lng - d, lat - d], [lng + d, lat - d], [lng + d, lat + d], [lng - d, lat + d], [lng - d, lat - d]]] });

const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
await call("POST", "/auth/otp/request", null, { phone: "+255626512332", app: "rider" });
const rt = (await call("POST", "/auth/otp/verify", null, { phone: "+255626512332", app: "rider", code: "0000" })).access_token;
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const tag = String(Date.now()).slice(-6);

// Route line for the zone: the quote's own road.
const q0 = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const line = decode(psql(`select route->>'polyline' from quotes where id='${q0.quote_id}'`));
if (line.length < 3) fail("routing returned no road line; ROUTE_CROSSES needs one");
const mid = line[Math.floor(line.length / 2)];

// 1) place at the drop-off, 2) zone on the road.
const place = await call("POST", "/admin/geofences", staff, { country: "TZ", kind: "PLACE", code: `e2e-airport-${tag}`, name: `E2E airport ${tag}`, center: DROPOFF, radius_m: 300, reason: "e2e" }, 201);
const zone = await call("POST", "/admin/geofences", staff, { country: "TZ", kind: "ZONE", code: `e2e-toll-${tag}`, name: `E2E toll ${tag}`, geometry: square(mid[0], mid[1], SQUARE_DEG), reason: "e2e" }, 201);
log(`geofences: place ${place.code} (r ${place.radius_m} m), zone ${zone.code} (${Math.round(zone.area_m2)} m², v${zone.version})`);

const profileOn = async (name, fence, charge) => {
  const p = await call("POST", "/admin/pricing/profiles", staff, { country: "TZ", name, reason: "e2e" }, 201);
  await call("POST", `/admin/pricing/profiles/${p.id}/versions`, staff, { terms: {}, charges: [charge], reason: "e2e" }, 201);
  await call("POST", `/admin/pricing/profiles/${p.id}/status`, staff, { status: "ACTIVE", reason: "e2e" });
  await call("POST", `/admin/pricing/profiles/${p.id}/assignments`, staff, { scope_kind: fence.kind, scope_id: fence.id, reason: "e2e" }, 201);
  return p;
};
const entry = await profileOn(`E2E airport ${tag}`, place, { code: "E2E_ENTRY", label: { en: "Airport entry", sw: "Ada ya kuingia uwanja" }, category: "ENTRY_FEE", amount_type: "FLAT", amount: String(ENTRY), recipient: "DRIVER", applies_when: "DROPOFF", on_fixed_routes: true });
const toll = await profileOn(`E2E toll ${tag}`, zone, { code: "E2E_TOLL", label: { en: "Road toll", sw: "Ushuru wa barabara" }, category: "TOLL", amount_type: "FLAT", amount: String(TOLL), recipient: "DRIVER", applies_when: "ROUTE_CROSSES", on_fixed_routes: true });
log("profiles applied: entry fee at the place (DROPOFF), toll on the zone (ROUTE_CROSSES)");

// 3) quote
await sleep(5500); // pricing and geofence caches check their versions every 5 s
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const charges = q.options[0].lines.filter((l) => l.code === "CHARGE");
log(`quote: ${charges.map((l) => `${l.label.en} ${l.amount}`).join(", ") || "no charges"}`);
const codes = charges.map((l) => l.meta.charge).sort().join(",");
if (codes !== "E2E_ENTRY,E2E_TOLL") fail(`expected the entry fee and the toll, got ${codes}`);
const trace = JSON.parse(psql(`select pricing->'trace' from quotes where id='${q.quote_id}'`));
if (!trace.dropoff?.candidates?.some((c) => c.scope.kind === "PLACE" && c.outcome === "WON")) fail("the drop-off trace must show the place winning");
if (!trace.crossed?.some((c) => c.scope.id === zone.id)) fail("the trace must list the crossed zone");
log("trace: the place won at the drop-off, the crossed zone is listed");

// 4) shape checks and import dry run
const bowtie = { type: "Polygon", coordinates: [[[39.20, -6.80], [39.21, -6.79], [39.21, -6.80], [39.20, -6.79], [39.20, -6.80]]] };
const bad = await call("POST", "/admin/geofences", staff, { country: "TZ", kind: "ZONE", code: `e2e-bad-${tag}`, name: "bad", geometry: bowtie, reason: "e2e" }, 400);
log(`self-intersecting outline → ${bad.error.code} (${bad.error.details?.reason})`);
const tiny = await call("POST", "/admin/geofences", staff, { country: "TZ", kind: "ZONE", code: `e2e-tiny-${tag}`, name: "tiny", geometry: square(-6.8, 39.25, 0.00001), reason: "e2e" }, 400);
log(`tiny outline → ${tiny.error.details?.reason}`);
const dry = await call("POST", "/admin/geofences/import", staff, {
  country: "TZ", kind: "ZONE", dry_run: true,
  features: { type: "FeatureCollection", features: [
    { properties: { name: "Import A", code: `e2e-imp-a-${tag}` }, geometry: square(-6.80, 39.27, 0.003) },
    { properties: { name: "Import B", code: `e2e-imp-b-${tag}` }, geometry: bowtie },
  ] },
});
log(`import dry run: ${dry.results.map((r) => `${r.code.split("-").slice(-2, -1)}=${r.outcome}`).join(", ")}`);
if (dry.results[0].outcome !== "CREATE" || dry.results[1].outcome !== "INVALID") fail("dry run must report CREATE then INVALID");
if (psql(`select count(*) from geofences where code like 'e2e-imp-%-${tag}'`) !== "0") fail("a dry run saved something");

// 5) tie: an overlapping zone whose profile has the same priority
const zone2 = await call("POST", "/admin/geofences", staff, { country: "TZ", kind: "ZONE", code: `e2e-toll2-${tag}`, name: `E2E toll 2 ${tag}`, geometry: square(mid[0], mid[1], SQUARE_DEG / 2), reason: "e2e" }, 201);
const p2 = await call("POST", "/admin/pricing/profiles", staff, { country: "TZ", name: `E2E toll 2 ${tag}`, reason: "e2e" }, 201);
await call("POST", `/admin/pricing/profiles/${p2.id}/versions`, staff, { terms: {}, charges: [], reason: "e2e" }, 201);
await call("POST", `/admin/pricing/profiles/${p2.id}/status`, staff, { status: "ACTIVE", reason: "e2e" });
const tie = await call("POST", `/admin/pricing/profiles/${p2.id}/assignments`, staff, { scope_kind: "ZONE", scope_id: zone2.id, reason: "e2e" }, 409);
log(`overlapping zone, same priority → ${tie.error.code}`);
await call("POST", `/admin/pricing/profiles/${p2.id}/assignments`, staff, { scope_kind: "ZONE", scope_id: zone2.id, priority: 5, reason: "e2e" }, 201);
log("with a higher priority it is accepted");

// Clean up: switch the profiles off (records keep them) and remove the shapes from the index.
for (const p of [entry, toll, p2]) {
  const full = await call("GET", `/admin/pricing/profiles/${p.id}`, staff);
  for (const s of full.scopes) await call("DELETE", `/admin/pricing/profiles/${p.id}/assignments/${s.assignment_id}?reason=e2e%20cleanup`, staff, undefined, 204);
  await call("POST", `/admin/pricing/profiles/${p.id}/status`, staff, { status: "INACTIVE", reason: "e2e cleanup" });
}
for (const f of [place, zone, zone2]) await call("PATCH", `/admin/geofences/${f.id}`, staff, { status: "ARCHIVED", reason: "e2e cleanup" });
log("GEOFENCES E2E PASSED");
