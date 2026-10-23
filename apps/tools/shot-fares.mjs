// Screenshots of the end-of-ride payment states in both web apps (rider :8083,
// driver :8084): cash due, a failed prompt, an open prompt (rider "approve on
// your phone", driver "waiting for approval"), then cash to finish the trip.
// The ride itself is driven through the API. Run: OUT=./tools/shots node tools/shot-fares.mjs
import { execFileSync } from "node:child_process";
import { chromium } from "playwright";

const out = process.env.OUT ?? "./tools/shots";
const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const psql = (sql) =>
  execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql], { stdio: ["ignore", "pipe", "pipe"] }).toString().trim();
async function call(method, path, token, body, headers = {}) {
  const r = await fetch(api + path, {
    method,
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...headers },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  if (!r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token;
};
const number = (suffix) => `07${String(Math.floor(Math.random() * 1e4)).padStart(4, "0")}${suffix}`;
const key = () => ({ "Idempotency-Key": `shot-${Date.now()}-${Math.random().toString(36).slice(2, 8)}` });

const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
const { trip: left } = await call("GET", "/trips/active", rt);
// A ride left by an interrupted run: cancel it, or finish it (rider pays cash, driver confirms).
if (left) {
  await call("POST", `/trips/${left.id}/cancel`, rt, { reason: "other" }).catch(async () => {
    let t = left;
    for (let next = t.allowed_transitions.find((x) => !x.startsWith("CANCELLED")); next; next = t.allowed_transitions.find((x) => !x.startsWith("CANCELLED"))) {
      if (next === "COMPLETED") await call("POST", `/trips/${t.id}/payment/cash`, rt, {}).catch(() => {});
      t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: next });
    }
  });
}
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: { lat: -6.7764, lng: 39.229 }, dropoff: { lat: -6.7724, lng: 39.2181 } });
let t = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" });
for (let i = 0; i < 30; i++) {
  const o = await call("GET", "/driver/offer", dt);
  if (o?.trip_id === t.id || o?.offer?.trip_id === t.id) break;
  await sleep(500);
}
t = await call("POST", `/driver/offers/${t.id}/accept`, dt);
for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT"]) {
  t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
}
log(`trip ${t.code} awaiting payment ${t.payment.amount}`);

const browser = await chromium.launch();
async function app(url, phone) {
  const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.7724, longitude: 39.2181 }, permissions: ["geolocation"] });
  const page = await ctx.newPage();
  await page.goto(url, { waitUntil: "load", timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").fill(phone);
  await page.locator('text="Send code" >> visible=true').click();
  await page.locator('text="Verify" >> visible=true').click();
  return page;
}
const rider = await app("http://localhost:8083/", "255626512332");
const driver = await app("http://localhost:8084/", "255715123702");
const shot = async (page, name, text) => {
  await page.getByText(text).first().waitFor({ timeout: 60000 }).catch(async (e) => {
    await page.screenshot({ path: `${out}/${name}-FAILED.png` });
    throw e;
  });
  await page.waitForTimeout(900);
  await page.screenshot({ path: `${out}/${name}.png` });
  log(`shot ${name}`);
};
await shot(rider, "fare-1-rider-cash", /Pay TZS/);
await call("POST", `/trips/${t.id}/payment/prompt`, rt, { phone: number("0001") }, key());
await shot(rider, "fare-2-rider-failed", "Payment didn't go through");
await call("POST", `/trips/${t.id}/payment/prompt`, rt, { phone: number("0003") }, key());
await shot(rider, "fare-3-rider-approve", "Approve on your phone");
await shot(driver, "fare-4-driver-waiting", "The rider is paying by mobile money");
psql(`update gateway_transactions set expires_at = now() - interval '1 second' where trip_id='${t.id}' and status='PENDING'`);
for (let i = 0; i < 30 && (await call("GET", `/trips/${t.id}`, rt)).payment.status !== "FAILED"; i++) await sleep(500);
await call("POST", `/trips/${t.id}/payment/cash`, rt, {});
await shot(driver, "fare-5-driver-cash", "The rider will pay you in cash.");
await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: "COMPLETED" });
await shot(rider, "fare-6-rider-done", "Thanks for riding");
await call("PUT", "/driver/status", dt, { online: false });
await browser.close();
