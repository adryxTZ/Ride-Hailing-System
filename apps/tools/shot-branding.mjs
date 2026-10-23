// Screenshots Settings › Branding (colours tab, a Color Hunt import, versions) and checks the page has no errors.
// Run: node tools/shot-branding.mjs
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1600, height: 1100 } })).newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 200)); });
page.on("pageerror", (e) => console.log("pageerror:", String(e).slice(0, 200)));
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
await page.goto("http://localhost:3000/settings/branding", { waitUntil: "networkidle", timeout: 180000 });
await page.waitForTimeout(2000);
await page.screenshot({ path: `${out}/branding-colours.png`, fullPage: true });
await page.getByLabel("Color Hunt link").fill("https://colorhunt.co/palette/0b244719376d576cbca5d7e8");
await page.getByRole("button", { name: "Import" }).click();
await page.waitForTimeout(1500);
await page.screenshot({ path: `${out}/branding-import.png` });
console.log("report:", await page.locator("aside").getByText(/Ready to publish|Fix the checks/).textContent());
await page.getByRole("tab", { name: "Versions" }).click();
await page.waitForTimeout(800);
await page.screenshot({ path: `${out}/branding-versions.png` });
await page.getByRole("tab", { name: "Identity" }).click();
await page.waitForTimeout(800);
await page.screenshot({ path: `${out}/branding-identity.png` });
await browser.close();
