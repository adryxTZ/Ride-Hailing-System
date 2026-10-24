// Screenshot of the admin Dispatch settings page (trip queue), signed in as staff.
// Run: ADMIN=http://localhost:3000 OUT=./tools/shots STAFF_EMAIL=… STAFF_PASSWORD=… node tools/shot-dispatch-settings.mjs
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const admin = process.env.ADMIN ?? "http://localhost:3000";
const out = process.env.OUT ?? "./tools/shots";
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 900 } })).newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 200)); });
await page.goto(`${admin}/login`, { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
await page.goto(`${admin}/settings/dispatch`, { waitUntil: "networkidle", timeout: 180000 });
await page.getByText("Offer the next trip while a driver is finishing").first().waitFor({ timeout: 30000 });
await page.waitForTimeout(1200);
await page.screenshot({ path: `${out}/admin-settings-dispatch.png`, fullPage: true });
// The audited change asks for a reason first.
await page.getByRole("switch").first().click();
await page.getByRole("dialog").waitFor();
await page.waitForTimeout(600); // the dialog's entrance
await page.screenshot({ path: `${out}/admin-settings-dispatch-reason.png` });
console.log("shot /settings/dispatch");
await browser.close();
