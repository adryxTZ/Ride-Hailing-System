// Screenshots of driver web pages (:8084), signed in as the test driver.
// Run: OUT=./tools/shots node tools/shot-driver.mjs /account/documents [/earnings ...]
import { chromium } from "playwright";
const out = process.env.OUT ?? "./tools/shots";
const paths = process.argv.slice(2).length ? process.argv.slice(2) : ["/"];
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.777, longitude: 39.23 }, permissions: ["geolocation"] });
const page = await ctx.newPage();
await page.goto("http://localhost:8084/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255715123702");
await page.locator('text="Send code" >> visible=true').click();
await page.locator('text="Verify" >> visible=true').click();
await page.waitForTimeout(3000);
for (const p of paths) {
  await page.goto(`http://localhost:8084${p}`, { waitUntil: "load" });
  await page.waitForTimeout(3500);
  await page.screenshot({ path: `${out}/driver${p.replace(/\//g, "-")}.png`, fullPage: true });
  console.log("shot", p);
}
await browser.close();
