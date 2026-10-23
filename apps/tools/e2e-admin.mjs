// Admin dashboard smoke test: signs in as the Super Admin test account, opens every page,
// checks its heading renders with no console errors, and runs the ledger integrity check.
// Run: node tools/e2e-admin.mjs   (admin on :3000, API on :8080).
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const BASE = process.env.ADMIN_URL ?? "http://localhost:3000";
const PAGES = [
  ["/", "Overview"], ["/live", "Live"], ["/trips", "Trips"], ["/drivers", "Drivers"], ["/riders", "Riders"],
  ["/pricing", "Vehicle types"], ["/pricing/taxi/standard", "Ride · Standard"], ["/areas", "Regions"], ["/pricing/fees", "Fees and tax"],
  ["/pricing/engines", "Engines"], ["/finance", "Finance"], ["/safety", "Safety alerts"], ["/audit", "Audit log"], ["/staff", "Staff"],
];

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const errors = [];
page.on("console", (m) => m.type() === "error" && errors.push(m.text().slice(0, 200)));
page.on("pageerror", (e) => errors.push(String(e).slice(0, 200)));

await page.goto(`${BASE}/login`, { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });

let failed = 0;
for (const [path, heading] of PAGES) {
  errors.length = 0;
  await page.goto(`${BASE}${path}`, { waitUntil: "networkidle", timeout: 180000 });
  const ok = await page.getByRole("heading", { level: 1, name: heading }).waitFor({ timeout: 15000 }).then(() => true, () => false);
  const pass = ok && errors.length === 0;
  if (!pass) failed++;
  console.log(`${pass ? "ok  " : "FAIL"} ${path}${ok ? "" : " (no heading)"}${errors.length ? ` errors: ${errors.join(" | ")}` : ""}`);
}

await page.goto(`${BASE}/pricing`, { waitUntil: "networkidle" });
const tariffs = await page.locator("table").first().locator("tbody tr").count();
console.log(`${tariffs === 5 ? "ok  " : "FAIL"} vehicle types table shows ${tariffs} ride types (want 5)`);
if (tariffs !== 5) failed++;
await page.locator("table").first().locator("tbody tr").first().click();
const typePage = await page.getByRole("button", { name: "Save prices" }).waitFor({ timeout: 15000 }).then(() => true, () => false);
console.log(`${typePage ? "ok  " : "FAIL"} a vehicle type opens its own settings page`);
if (!typePage) failed++;
await page.goto(`${BASE}/areas`, { waitUntil: "networkidle" });
const regions = await page.getByRole("button", { name: /Dar es Salaam/ }).waitFor({ timeout: 15000 }).then(() => true, () => false);
console.log(`${regions ? "ok  " : "FAIL"} regions page lists Dar es Salaam`);
if (!regions) failed++;

await page.goto(`${BASE}/finance`, { waitUntil: "networkidle" });
const verifyRes = page.waitForResponse((r) => r.url().endsWith("/admin/finance/verify"));
await page.getByRole("button", { name: "Verify ledger" }).click();
const vr = await verifyRes;
const vbody = await vr.text();
const intact = await page.getByText(/Ledger intact/).waitFor({ timeout: 15000 }).then(() => true, () => false);
console.log(`${intact ? "ok  " : "FAIL"} ledger verifies${intact ? "" : ` (${vr.status()} ${vbody.slice(0, 200)})`}`);
if (!intact) failed++;

await browser.close();
console.log(failed ? `${failed} check(s) failed` : "all admin checks passed");
process.exit(failed ? 1 : 0);
