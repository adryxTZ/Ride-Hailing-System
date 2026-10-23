// Boda package delivery across both web apps: rider taps "Package" → route → receiver + what is
// sent → options (Boda) → confirm; the boda driver gets "Package delivery for …", accepts and
// goes Collect → Package collected → At the receiver → Delivered → Cash collected; the trip
// carries the receiver and the note.
// Run: OUT=./tools/shots node tools/e2e-web-package.mjs  (rider :8083, driver :8084, API :8080)
import { chromium } from "playwright";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
execFileSync(fileURLToPath(new URL("../../backend/bin/nexa", import.meta.url)),
  ["driver", "provision", "+255715123702", "--service", "taxi", "--class", "boda", "--plate", "MC123ABC", "--name", "Adrian Mvungi"],
  { cwd: fileURLToPath(new URL("../../backend/", import.meta.url)), env: { ...process.env, NEXA_BRAND: "nexa", NEXA_DATABASE_URL: "postgres://nexa:nexa@localhost:5433/nexa?sslmode=disable", NEXA_REDIS_URL: "redis://localhost:6380/0", NEXA_LOG_LEVEL: "warn" } });
const post = async (path, body, token) => (await fetch(api + path, { method: "POST", headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: JSON.stringify(body) })).json();
await post("/auth/otp/request", { phone: "+255626512332", app: "rider" });
const { access_token: rt } = await post("/auth/otp/verify", { phone: "+255626512332", code: "0000", app: "rider" });
// Going online again picks up the Boda (presence keeps what the driver went online with).
await post("/auth/otp/request", { phone: "+255715123702", app: "driver" });
const { access_token: dtok } = await post("/auth/otp/verify", { phone: "+255715123702", code: "0000", app: "driver" });
await fetch(`${api}/driver/status`, { method: "PUT", headers: { ...j, Authorization: `Bearer ${dtok}` }, body: JSON.stringify({ online: false }) });
const { trip: old } = await (await fetch(`${api}/trips/active`, { headers: { Authorization: `Bearer ${rt}` } })).json();
if (old) await post(`/trips/${old.id}/cancel`, { reason: "other" }, rt);

const browser = await chromium.launch();
async function app(url, phone, geo, name) {
  const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: geo, permissions: ["geolocation"] });
  const page = await ctx.newPage();
  await page.goto(url, { waitUntil: "load", timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").fill(phone);
  await page.getByText(/^Send code$/).locator("visible=true").first().click();
  await page.getByText(/^Verify$/).locator("visible=true").first().click();
  const vis = (re) => page.getByText(re).locator("visible=true").first();
  return { page, vis, shot: (n) => page.screenshot({ path: `${out}/${n}.png` }), name };
}
const driver = await app("http://localhost:8084/", "255715123702", { latitude: -6.7770, longitude: 39.2300 }, "driver");
const state = await Promise.race([
  driver.vis(/^Go online$/).waitFor({ timeout: 60000 }).then(() => "offline"),
  driver.vis(/^Finding trips near you$/).waitFor({ timeout: 60000 }).then(() => "online"),
]);
if (state === "offline") { await driver.page.waitForTimeout(600); await driver.vis(/^Go online$/).click(); }
await driver.vis(/^Finding trips near you$/).waitFor({ timeout: 30000 }); log("boda driver online");

const rider = await app("http://localhost:8083/", "255626512332", { latitude: -6.7764, longitude: 39.229 }, "rider");
process.on("uncaughtException", async (e) => { await rider.shot("package-fail-rider").catch(() => {}); await driver.shot("package-fail-driver").catch(() => {}); console.error(e.message.split("\n")[0]); process.exit(1); });
await rider.vis(/^(Package|Kifurushi)$/).waitFor({ timeout: 60000 }); await rider.page.waitForTimeout(800);
await rider.shot("pkg-0-home");
await rider.vis(/^(Package|Kifurushi)$/).click();
await rider.vis(/^Route$/).waitFor({ timeout: 20000 });
await rider.vis(/^Gym$/).waitFor({ timeout: 20000 }); await rider.vis(/^Gym$/).click();
await rider.vis(/^Package details$/).waitFor({ timeout: 20000 }); await rider.page.waitForTimeout(1200);
await rider.page.getByPlaceholder("Name").fill("Asha Mussa");
await rider.page.getByPlaceholder("07XX XXX XXX").fill("0712 345 678");
await rider.page.getByPlaceholder("e.g. documents, a phone, keys").fill("Envelope with documents");
await rider.shot("pkg-1-details");
await rider.vis(/^Continue$/).click();
await rider.vis(/^Confirm order$/).waitFor({ timeout: 25000 }); await rider.page.waitForTimeout(2000);
await rider.shot("pkg-2-options");
await rider.vis(/^Confirm order$/).click(); log("package booked");

await driver.vis(/^Package delivery for /).waitFor({ timeout: 25000 }); await driver.page.waitForTimeout(600);
await driver.shot("pkg-3-driver-offer"); log("driver sees: Package delivery");
await driver.vis(/^Accept$/).click();
for (const [button, shot] of [[/^I've arrived$/, null], [/^Package collected$/, "pkg-4-driver-collect"], [/^At the receiver$/, null], [/^Delivered$/, "pkg-5-driver-receiver"], [/^Cash collected · TZS/, null]]) {
  await driver.vis(button).waitFor({ timeout: 20000 });
  if (shot) { await driver.page.waitForTimeout(500); await driver.shot(shot); }
  await driver.vis(button).click(); log(`driver: ${button}`);
  await driver.page.waitForTimeout(700);
}
const trips = (await (await fetch(`${api}/trips`, { headers: { Authorization: `Bearer ${rt}` } })).json()).data;
const t = trips[0];
log(`trip ${t.service} ${t.status}, receiver ${t.contacts?.dropoff?.name}, note "${t.notes}"`);
if (t.service !== "parcel" || t.status !== "COMPLETED" || t.contacts?.dropoff?.name !== "Asha Mussa") throw new Error("package trip not as expected");
log("PACKAGE E2E PASSED");
await browser.close();
