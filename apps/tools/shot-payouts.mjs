// Screenshots of cash-outs: the driver's Earnings card (web :8084) and the admin
// Finance page (:3000) with a request waiting for approval, then the request is
// withdrawn. Earns on mobile-money rides first when the driver has too little.
// Run: OUT=./tools/shots node tools/shot-payouts.mjs
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const out = process.env.OUT ?? "./tools/shots";
const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function call(method, path, token, body, headers = {}) {
  const r = await fetch(api + path, { method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...headers }, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (!r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token;
};
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
async function mobileMoneyRide() {
  await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
  const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
  const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: { lat: -6.7764, lng: 39.229 }, dropoff: { lat: -6.7724, lng: 39.2181 } });
  let t = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "MOBILE_MONEY" });
  for (let i = 0; i < 30; i++) {
    const o = await call("GET", "/driver/offer", dt);
    if (o?.trip_id === t.id || o?.offer?.trip_id === t.id) break;
    await sleep(500);
  }
  t = await call("POST", `/driver/offers/${t.id}/accept`, dt);
  for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT"]) t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
  for (let i = 0; i < 40 && (await call("GET", `/trips/${t.id}`, rt)).status !== "COMPLETED"; i++) await sleep(500);
  await call("PUT", "/driver/status", dt, { online: false });
}
let w = await call("GET", "/driver/wallet", dt);
for (let i = 0; i < 4 && Number(w.payouts.available_payout) < 9000; i++) {
  await mobileMoneyRide();
  w = await call("GET", "/driver/wallet", dt);
}
log(`available ${w.payouts.available_payout}`);

const browser = await chromium.launch();
const driverCtx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.777, longitude: 39.23 }, permissions: ["geolocation"] });
const driver = await driverCtx.newPage();
await driver.goto("http://localhost:8084/", { waitUntil: "load", timeout: 240000 });
await driver.getByPlaceholder("07XX XXX XXX").fill("255715123702");
await driver.locator('text="Send code" >> visible=true').click();
await driver.locator('text="Verify" >> visible=true').click();
await driver.waitForTimeout(3000);
await driver.goto("http://localhost:8084/earnings", { waitUntil: "load" });
await driver.getByText("Cash out").first().waitFor({ timeout: 60000 });
await driver.getByText("Cash out", { exact: true }).last().scrollIntoViewIfNeeded();
await driver.waitForTimeout(900);
await driver.screenshot({ path: `${out}/payout-1-driver-card.png` });
await driver.locator('text="Cash out" >> visible=true').last().click();
await driver.getByText(/^Cash out TZS/).waitFor({ timeout: 20000 });
await driver.waitForTimeout(600);
await driver.screenshot({ path: `${out}/payout-2-driver-sheet.png` });
await driver.getByText(/^Cash out TZS/).click();
await driver.getByText("Waiting for approval").first().waitFor({ timeout: 20000 });
await driver.waitForTimeout(600);
await driver.screenshot({ path: `${out}/payout-3-driver-requested.png` });
log("driver shots");

const admin = await (await browser.newContext({ viewport: { width: 1440, height: 1100 } })).newPage();
await admin.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await admin.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await admin.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await admin.getByRole("button", { name: "Sign in" }).click();
await admin.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
await admin.goto("http://localhost:3000/finance?country=TZ", { waitUntil: "networkidle" });
await admin.getByText("Cash-outs").first().waitFor({ timeout: 60000 });
await admin.waitForTimeout(2000);
await admin.screenshot({ path: `${out}/payout-4-admin-finance.png`, fullPage: true });
log("admin shot");
const open = (await call("GET", "/driver/payouts", dt)).data.find((p) => p.status === "REQUESTED");
if (open) await call("POST", `/driver/payouts/${open.id}/cancel`, dt, {});
await browser.close();
