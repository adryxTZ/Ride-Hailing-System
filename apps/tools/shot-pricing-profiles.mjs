// Screenshots the pricing profiles list, a profile's tabs and a trip's pricing record.
// Run: node tools/shot-pricing-profiles.mjs <profileId> <tripId>
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const [, , profileId, tripId] = process.argv;
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1600, height: 1100 } })).newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 200)); });
page.on("pageerror", (e) => console.log("pageerror:", String(e).slice(0, 200)));
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
const shot = async (path, name) => {
  await page.goto(`http://localhost:3000${path}`, { waitUntil: "networkidle", timeout: 180000 });
  await page.waitForTimeout(2000);
  await page.screenshot({ path: `${out}/${name}.png`, fullPage: true });
  console.log("shot", name);
};
await shot("/pricing/fees", "profiles-list");
await shot("/pricing/fees?tab=payees", "profiles-payees");
for (const tab of ["rules", "charges", "where", "versions", "activity"]) await shot(`/pricing/fees/${profileId}?tab=${tab}`, `profile-${tab}`);
await page.goto(`http://localhost:3000/pricing/fees/${profileId}?tab=charges`, { waitUntil: "networkidle" });
await page.getByRole("button", { name: /City levy/ }).click();
await page.waitForTimeout(600);
await page.screenshot({ path: `${out}/profile-charge-open.png`, fullPage: true });
await shot(`/trips/${tripId}`, "trip-pricing-record");
await browser.close();
