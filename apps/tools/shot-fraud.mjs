// Screenshots the fraud queue (all statuses) and a GPS and a self-booking flag. Run: node tools/shot-fraud.mjs <gpsFlagId> <selfFlagId>
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const [, , gps, self] = process.argv;
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1600, height: 1000 } })).newPage();
page.on("pageerror", (e) => console.log("pageerror:", String(e).slice(0, 200)));
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
await page.goto("http://localhost:3000/compliance/fraud?status=ALL", { waitUntil: "networkidle" });
await page.waitForTimeout(1500);
await page.screenshot({ path: `${out}/fraud-queue.png` });
for (const [name, id] of [["gps", gps], ["self", self]]) {
  await page.goto(`http://localhost:3000/compliance/fraud/${id}`, { waitUntil: "networkidle" });
  await page.waitForTimeout(2500);
  await page.screenshot({ path: `${out}/fraud-${name}.png`, fullPage: true });
}
await browser.close();
console.log("done");
