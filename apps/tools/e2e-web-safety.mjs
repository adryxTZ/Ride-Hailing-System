// Safety screens (rider web): Account → Safety saves two trusted contacts; on an accepted trip the
// Safety pill opens the sheet (share, alert contacts, call 112). SOS itself is covered by e2e-safety.mjs
// (its confirm dialog is native-only).
// Run: OUT=./tools/shots node tools/e2e-web-safety.mjs  (rider web :8083, API :8080)
import { chromium } from "playwright";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
async function call(method, path, token, body) {
  const r = await fetch(api + path, { method, headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const t = await r.text(); if (!r.ok) throw new Error(`${method} ${path} → ${r.status} ${t.slice(0, 160)}`); return t ? JSON.parse(t) : null;
}
const sign = async (phone, app) => { await call("POST", "/auth/otp/request", null, { phone, app }); return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token; };
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
await call("PUT", "/me/trusted-contacts", rt, { data: [] });
const { trip: old } = await call("GET", "/trips/active", rt); if (old) await call("POST", `/trips/${old.id}/cancel`, rt, { reason: "other" });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.7764, longitude: 39.229 }, permissions: ["geolocation"] });
const page = await ctx.newPage();
const vis = (re) => page.getByText(re).locator("visible=true").first();
process.on("uncaughtException", async (e) => { await page.screenshot({ path: `${out}/safety-fail.png` }).catch(() => {}); console.error(e.message.split("\n")[0]); process.exit(1); });
await page.goto("http://localhost:8083/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255626512332");
await vis(/^Send code$/).click(); await vis(/^Verify$/).click();
await vis(/^Where to\?$/).waitFor({ timeout: 60000 });

await page.goto("http://localhost:8083/account/safety", { waitUntil: "load" });
await vis(/^Add a trusted contact$/).click(); await vis(/^Add a trusted contact$/).click();
const names = page.getByPlaceholder("Name"), phones = page.getByPlaceholder("07XX XXX XXX");
await names.nth(0).fill("Mama Neema"); await phones.nth(0).fill("0712 000 001");
await names.nth(1).fill("Juma"); await phones.nth(1).fill("0754 000 002");
await vis(/^Save$/).click(); await vis(/^Saved$/).waitFor({ timeout: 10000 });
await page.screenshot({ path: `${out}/safety-1-contacts.png` });
log(`saved: ${(await call("GET", "/me/trusted-contacts", rt)).data.map((c) => c.phone).join(", ")}`);

await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.7770, lng: 39.2300 } });
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: { lat: -6.7764, lng: 39.229 }, dropoff: { lat: -6.7724, lng: 39.2181 } });
const t = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" });
for (let i = 0; i < 20; i++) { const o = await call("GET", "/driver/offer", dt); if (o?.trip_id === t.id || o?.offer?.trip_id === t.id) break; await new Promise((r) => setTimeout(r, 500)); }
await call("POST", `/driver/offers/${t.id}/accept`, dt);
await page.goto(`http://localhost:8083/book/requested?tripId=${t.id}`, { waitUntil: "load" });
await vis(/^Safety$/).waitFor({ timeout: 20000 }); await page.waitForTimeout(2500); await vis(/^Safety$/).click();
await vis(/^Share trip status$/).waitFor({ timeout: 10000 });
await vis(/^Call emergency services · 112$/).waitFor({ timeout: 10000 });
await page.waitForTimeout(600); await page.screenshot({ path: `${out}/safety-2-sheet.png` });
log("safety sheet: share, alert contacts, call 112");
await call("POST", `/trips/${t.id}/cancel`, rt, { reason: "other" });
await call("PUT", "/driver/status", dt, { online: false });
log("SAFETY UI E2E PASSED");
await browser.close();
