// Picks Kenya then Nairobi in the top-bar picker and screenshots each operations page,
// to check the whole dashboard follows the chosen country. Run: node tools/shot-kenya.mjs
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1000 } })).newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 200)); });
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
await page.goto("http://localhost:3000/live", { waitUntil: "networkidle" });
await page.getByLabel("Country").selectOption("KE");
await page.waitForTimeout(1500);
const region = page.getByLabel(/County|Region/).first();
const nairobi = await region.locator("option", { hasText: "Nairobi" }).getAttribute("value");
await region.selectOption(nairobi);
await page.waitForTimeout(2500);
for (const p of ["/live", "/", "/trips", "/pricing", "/areas", "/drivers", "/finance"]) {
  await page.goto(`http://localhost:3000${p}`, { waitUntil: "networkidle", timeout: 180000 });
  await page.waitForTimeout(2500);
  const name = p === "/" ? "overview" : p.slice(1);
  await page.screenshot({ path: `${out}/ke-${name}.png` });
  console.log("shot", p, await page.getByLabel("Country").inputValue());
}
await browser.close();
