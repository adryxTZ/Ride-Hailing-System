// Safety against the real API: trusted contacts (max 3, phones normalised), share link (public,
// no phone numbers or fares), SOS (texts contacts via the SMS provider; staff queue; resolve).
// Run: node tools/e2e-safety.mjs  (API on :8080)
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, { method, headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => { await call("POST", "/auth/otp/request", null, { phone, app }); return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token; };
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver"), st = await sign("+255626512332", "admin");
await call("PUT", "/me/trusted-contacts", rt, { data: [1, 2, 3, 4].map((i) => ({ name: `C${i}`, phone: `071200000${i}` })) }, 400);
const saved = await call("PUT", "/me/trusted-contacts", rt, { data: [{ name: "Mama", phone: "0712 000 001" }, { name: "Juma", phone: "0754000002" }] });
log(`trusted contacts: ${saved.data.map((c) => `${c.name} ${c.phone}`).join(", ")}`);

const { trip: old } = await call("GET", "/trips/active", rt);
if (old) await call("POST", `/trips/${old.id}/cancel`, rt, { reason: "other" });
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.7770, lng: 39.2300 } });
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: { lat: -6.7764, lng: 39.229 }, dropoff: { lat: -6.7724, lng: 39.2181 } });
let t = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
for (let i = 0; i < 20; i++) { const o = await call("GET", "/driver/offer", dt); if (o?.trip_id === t.id || o?.offer?.trip_id === t.id) break; await new Promise((r) => setTimeout(r, 500)); }
t = await call("POST", `/driver/offers/${t.id}/accept`, dt);
await call("POST", "/driver/location", dt, { location: { lat: -6.7768, lng: 39.2296 }, heading: 90 });

const link = await call("POST", `/trips/${t.id}/share`, rt);
log(`share link: ${link.url}`);
const view = await call("GET", `/share/${link.token}`, null);
log(`public view: ${view.status}, rider ${view.rider_first_name}, driver ${view.driver_first_name}, vehicle "${view.vehicle}", driver at ${view.driver_location?.lat?.toFixed(4)}`);
if (JSON.stringify(view).match(/\+255|fare/)) throw new Error("public view leaks a phone number or a fare");
const page = await fetch(link.url); log(`share page: ${page.status} ${page.headers.get("content-type")}`);
await call("GET", "/share/not-a-token", null, null, 404);

const alert = await call("POST", `/trips/${t.id}/sos`, rt, { location: { lat: -6.7766, lng: 39.2292 } }, 201);
log(`SOS raised: ${alert.contacts_alerted} contacts texted`);
const queue = await call("GET", "/admin/sos", st);
log(`staff queue has the alert: ${queue.data.some((a) => a.id === alert.id)}`);
const events = await call("GET", `/trips/${t.id}/events`, rt);
log(`trip log has SOS: ${JSON.stringify(events).includes("trip.sos")}`);
await call("POST", `/admin/sos/${alert.id}/resolve`, st, { resolution: "Called the rider; all fine" }, 204);
log(`after resolve, open alerts with it: ${(await call("GET", "/admin/sos", st)).data.some((a) => a.id === alert.id)}`);
await call("POST", `/trips/${t.id}/cancel`, rt, { reason: "other" });
await call("PUT", "/driver/status", dt, { online: false });
log("SAFETY E2E PASSED");
