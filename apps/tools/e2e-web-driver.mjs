// Smoke test: driver web build sign-in → go online → GPS ping → go offline.
// Run: OUT=./tools/shots node tools/e2e-web-driver.mjs   (needs `cd driver && BRAND=nexa API_BASE_URL=http://localhost:8080/v1 npx expo start --web --port 8084`)
import { chromium } from "playwright";
const out = process.env.OUT ?? ".";
const base = process.env.WEB ?? "http://localhost:8084";
const browser = await chromium.launch();
const ctx = await browser.newContext({
  viewport: { width: 420, height: 820 },
  geolocation: { latitude: -6.8161, longitude: 39.2894 },
  permissions: ["geolocation"],
});
const page = await ctx.newPage();
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
page.on("console", (msg) => { if (msg.type() === "error") log("console.error: " + msg.text().slice(0, 200)); });
await page.goto(base + "/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255715123702");
await page.getByText("Send code", { exact: true }).click();
await page.getByText("Enter the code").waitFor({ timeout: 20000 });
await page.getByText("Verify", { exact: true }).click(); log("verifying");
await page.getByText("You're offline", { exact: true }).waitFor({ timeout: 60000 });
await page.screenshot({ path: `${out}/d1-offline.png`, fullPage: true }); log("home: offline state rendered");
await page.getByText("Go online", { exact: true }).click();
await page.getByText("Finding trips near you", { exact: true }).waitFor({ timeout: 20000 });
await page.screenshot({ path: `${out}/d2-online.png`, fullPage: true }); log("went online");
await page.waitForTimeout(12000); // one stream interval
const cell = await page.getByText(/^\d\d:\d\d$/).first().textContent().catch(() => null);
log("online timer shows: " + cell); await page.screenshot({ path: `${out}/d3-online-later.png`, fullPage: true });
await page.getByRole("switch").click();
await page.getByText("You're offline", { exact: true }).waitFor({ timeout: 20000 }); log("went offline");
await browser.close();
