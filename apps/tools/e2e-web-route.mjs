// Route screen check (rider web): the pickup gets a street name from the GPS position,
// "+" adds a stop filled by typing a place name, the drop-off is found by search, and
// the booked trip carries PICKUP → WAYPOINT → DROPOFF in order.
// Run: OUT=./tools/shots node tools/e2e-web-route.mjs  (rider web on :8083)
import { chromium } from "playwright";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const post = async (path, body, token) => (await fetch(api + path, { method: "POST", headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: JSON.stringify(body) })).json();
await post("/auth/otp/request", { phone: "+255626512332", app: "rider" });
const { access_token: rt } = await post("/auth/otp/verify", { phone: "+255626512332", code: "0000", app: "rider" });
const auth = { Authorization: `Bearer ${rt}` };
const { trip: old } = await (await fetch(`${api}/trips/active`, { headers: auth })).json();
if (old) await post(`/trips/${old.id}/cancel`, { reason: "other", note: "e2e cleanup" }, rt);

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.7764, longitude: 39.229 }, permissions: ["geolocation"] });
const page = await ctx.newPage();
const vis = (t) => page.locator(`text="${t}" >> visible=true`).first();
await page.goto("http://localhost:8083/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255626512332");
await vis("Send code").click(); await vis("Verify").click();
await vis("Where to?").click();
await vis("Route").waitFor({ timeout: 20000 });
await page.waitForTimeout(2500);
const pickup = await page.locator("text=/Chui Street|Current location/ >> visible=true").first().textContent().catch(() => "?");
log(`pickup label: ${pickup}`);
await page.getByLabel("Add stop").click();
await page.getByPlaceholder("Stop 1").fill("sinza");
await page.getByText(/Sinza/).locator("visible=true").first().waitFor({ timeout: 20000 });
await page.screenshot({ path: `${out}/route-1-stop-search.png` });
const stopName = await page.getByText(/Sinza/).locator("visible=true").first().textContent();
await page.getByText(/Sinza/).locator("visible=true").first().click(); log(`stop chosen: ${stopName}`);
await page.getByPlaceholder("Dropoff location").fill("mlimani");
await page.getByText(/^Mlimani City/).locator("visible=true").first().waitFor({ timeout: 20000 });
await page.screenshot({ path: `${out}/route-2-dropoff-search.png` });
await page.getByText(/^Mlimani City/).locator("visible=true").first().click(); log("drop-off chosen");
await vis("Confirm order").waitFor({ timeout: 25000 }); await page.waitForTimeout(2000);
await page.screenshot({ path: `${out}/route-3-options.png` });
await page.getByText(/^(Kawaida|Standard)$/).locator("visible=true").first().click();
await vis("Confirm order").click();
await page.getByText(/^(Looking for a driver|Ride requested)$/).locator("visible=true").first().waitFor({ timeout: 20000 });
const { trip } = await (await fetch(`${api}/trips/active`, { headers: auth })).json();
log("trip stops: " + trip.stops.map((s) => `${s.kind}(${s.address})`).join(" → "));
if (trip.stops.map((s) => s.kind).join(",") !== "PICKUP,WAYPOINT,DROPOFF") throw new Error("stops not saved in order");
await post(`/trips/${trip.id}/cancel`, { reason: "other", note: "e2e" }, rt);
log("ROUTE E2E PASSED");
await browser.close();
