// Driver web: on hold → Home shows the hold panel (no "Go online") → Pay now → USSD sheet → "Payment received" → hold gone.
// Run: OUT=./tools/shots node tools/e2e-web-hold.mjs   (API on :8080 with the fake gateway; driver web on :8084)
import { chromium } from "playwright";
const out = process.env.OUT ?? "./tools/shots";
const base = process.env.WEB ?? "http://localhost:8084";
import { execFileSync } from "node:child_process";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const psql = (sql) => execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql]).toString().trim();
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, { method, headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => { await call("POST", "/auth/otp/request", null, { phone, app }); return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token; };
const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver"), st = await sign("+255626512332", "admin");
const DEFAULTS = { debt_soft_limit: "1000", debt_hard_limit: "5000", debt_soft_grace_days: 7, payin_min_amount: "1000" };
const policyOf = async () => (await call("GET", "/admin/pricing", st)).policy;
const publish = (changes, reason) => policyOf().then((p) => call("POST", "/admin/pricing/policy", st, { ...p, ...changes, reason }));
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
// Prompts are rate-limited per driver and per number; age the test driver's earlier pushes so this run is not refused.
psql("update gateway_transactions set created_at = created_at - interval '2 days' where payer_id = (select id from users where phone='+255715123702')");

// Arrange: strict limits and enough cash-trip debt to be held.
await call("PUT", "/driver/status", dt, { online: false });
await publish({ ...DEFAULTS, debt_hard_limit: "1000" }, "web e2e: strict limits");
await sleep(6000); // the pricing cache re-checks the config version every 5 s
let wallet = await call("GET", "/driver/wallet", dt);
for (let i = 0; Number(wallet.debt) < 1000 && i < 8; i++) {
  await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
  const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
  const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
  let t = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
  for (let k = 0; k < 20; k++) { const o = await call("GET", "/driver/offer", dt); if (o?.offer?.trip_id === t.id || o?.trip_id === t.id) break; await sleep(500); }
  t = await call("POST", `/driver/offers/${t.id}/accept`, dt);
  let next = t.allowed_transitions.find((s) => !s.startsWith("CANCELLED"));
  while (next) { t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: next }); next = t.allowed_transitions.find((s) => !s.startsWith("CANCELLED")); }
  await sleep(800);
  wallet = await call("GET", "/driver/wallet", dt);
}
if (!wallet.holds.length) throw new Error(`driver not on hold (debt ${wallet.debt})`);
log(`driver on hold: owes ${wallet.debt}`);

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 420, height: 820 }, geolocation: { latitude: -6.8161, longitude: 39.2894 }, permissions: ["geolocation"] });
const page = await ctx.newPage();
page.on("console", (msg) => { if (msg.type() === "error") log("console.error: " + msg.text().slice(0, 200)); });
try {
  await page.goto(base + "/", { waitUntil: "load", timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").fill("255715123702");
  await page.getByText("Send code", { exact: true }).click();
  await page.getByText("Enter the code").waitFor({ timeout: 20000 });
  await page.getByText("Verify", { exact: true }).click();
  await page.getByText("On hold: pay to go online", { exact: true }).waitFor({ timeout: 60000 });
  if (await page.getByText("Go online", { exact: true }).count()) throw new Error("Go online must not show while held");
  if (await page.getByRole("switch").count()) throw new Error("online switch must be hidden while held");
  await page.waitForTimeout(600); // entrance animation
  await page.screenshot({ path: `${out}/h1-hold-home.png`, fullPage: true }); log("home: hold panel instead of Go online");
  // The floating tab bar's wrapper overlaps the sheet on web; force the taps like a finger would land them.
  await page.getByText("Pay now", { exact: true }).first().click({ force: true });
  await page.getByText("You owe", { exact: true }).waitFor({ timeout: 20000 }); log("earnings screen");
  await page.waitForTimeout(600);
  await page.screenshot({ path: `${out}/h2-earnings-hold.png`, fullPage: true });
  // Arriving with ?pay=1 opens the sheet by itself; otherwise tap the banner's button
  // (tabs stay mounted, so index 0 is Home's hidden button and index 1 the banner's).
  if (!(await page.getByText("Pay what you owe", { exact: true }).count())) {
    await page.locator("text=Pay now >> visible=true").nth(1).click({ force: true });
  }
  await page.getByText("Pay what you owe", { exact: true }).waitFor({ timeout: 20000 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${out}/h2-pay-sheet.png`, fullPage: true }); log("pay sheet open");
  await page.getByText(/^Pay TZS/).click({ force: true });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${out}/h3b-after-pay-tap.png`, fullPage: true });
  log(`after tap: pending=${await page.getByText("Check your phone", { exact: true }).count()} failed=${await page.getByText("Payment not completed", { exact: true }).count()} sheet=${await page.getByText("Pay what you owe", { exact: true }).count()}`);
  await page.getByText("Check your phone", { exact: true }).waitFor({ timeout: 20000 });
  await page.screenshot({ path: `${out}/h3-pay-pending.png`, fullPage: true }); log("waiting for the PIN");
  await page.getByText("Payment received", { exact: true }).waitFor({ timeout: 30000 });
  await page.screenshot({ path: `${out}/h4-pay-success.png`, fullPage: true }); log("payment received");
  await page.getByText("Done", { exact: true }).click({ force: true });
  await page.getByText("Your account is on hold", { exact: true }).waitFor({ state: "detached", timeout: 20000 }); log("hold banner gone");
  wallet = await call("GET", "/driver/wallet", dt);
  if (wallet.holds.length) throw new Error("hold still active after payment");
  log("WEB HOLD E2E PASSED");
} finally {
  await browser.close();
  await publish(DEFAULTS, "web e2e: restore limits");
}
