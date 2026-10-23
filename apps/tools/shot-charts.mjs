// Screenshots the Charts tab of a driver and a rider page, and the sidebar with
// one group open after opening another. Run: node tools/shot-charts.mjs <driverId> <riderId>
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const [, , driverId, riderId] = process.argv;
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1600, height: 1000 } })).newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 200)); });
page.on("pageerror", (e) => console.log("pageerror:", String(e).slice(0, 200)));
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
for (const [kind, id] of [["driver", driverId], ["rider", riderId]]) {
  await page.goto(`http://localhost:3000/${kind}s/${id}`, { waitUntil: "networkidle", timeout: 180000 });
  await page.getByRole("tab", { name: "Charts" }).click();
  await page.waitForTimeout(3500);
  console.log(kind, "url:", new URL(page.url()).search);
  await page.screenshot({ path: `${out}/charts-${kind}.png`, fullPage: true });
}
// Accordion: open Pricing, then Money; only Money should stay open.
await page.locator("nav[aria-label=Sections] button", { hasText: "Pricing" }).click();
await page.waitForTimeout(400);
await page.locator("nav[aria-label=Sections] button", { hasText: "Money" }).click();
await page.waitForTimeout(400);
const expanded = await page.locator("nav[aria-label=Sections] button[aria-expanded=true]").allTextContents();
console.log("open groups:", expanded.map((t) => t.trim()));
await page.screenshot({ path: `${out}/sidebar-accordion.png` });
await browser.close();
