// Resume check: a ride booked elsewhere (here: straight through the API) must take over
// the rider app on launch — the app opens on the live trip, not the home map.
// Run: OUT=./tools/shots node tools/e2e-web-resume.mjs  (rider web on :8083)
import { chromium } from "playwright";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const post = async (path, body, token) => (await fetch(api + path, { method: "POST", headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: JSON.stringify(body) })).json();
await post("/auth/otp/request", { phone: "+255626512332", app: "rider" });
const { access_token: rt } = await post("/auth/otp/verify", { phone: "+255626512332", code: "0000", app: "rider" });
const { trip: old } = await (await fetch(`${api}/trips/active`, { headers: { Authorization: `Bearer ${rt}` } })).json();
if (old) await post(`/trips/${old.id}/cancel`, { reason: "other", note: "e2e cleanup" }, rt);
const q = await post("/quotes", { service: "taxi", pickup: { lat: -6.7764, lng: 39.229 }, dropoff: { lat: -6.7724, lng: 39.2181 } }, rt);
const trip = await post("/trips", { quote_id: q.quote_id, vehicle_class: "xl", payment_method: "CASH", stops: [{ kind: "PICKUP", address: "Resume pickup" }, { kind: "DROPOFF", address: "Resume dropoff" }] }, rt);
log(`booked ${trip.code} outside the app`);

const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 412, height: 915 } })).newPage();
await page.goto("http://localhost:8083/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255626512332");
await page.locator('text="Send code" >> visible=true').click();
await page.locator('text="Verify" >> visible=true').click();
await page.getByText(/^(Looking for a driver|Ride requested)$/).locator("visible=true").first().waitFor({ timeout: 20000 });
await page.screenshot({ path: `${out}/resume.png` });
log("app opened straight on the live trip");
await post(`/trips/${trip.id}/cancel`, { reason: "other", note: "e2e" }, rt);
await page.locator('text="Where to?" >> visible=true').waitFor({ timeout: 20000 });
log("after the cancel (no booking draft) the app returned home");
log("RESUME E2E PASSED");
await browser.close();
