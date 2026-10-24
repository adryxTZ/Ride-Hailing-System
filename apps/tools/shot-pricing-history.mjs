// Screenshots the pricing simulator (with a trip simulated) and the history tabs. Run: node tools/shot-pricing-history.mjs
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const width = Number(process.env.WIDTH ?? 1600);
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width, height: 1000 } })).newPage();
page.on("pageerror", (e) => console.log("pageerror:", String(e).slice(0, 200)));
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 160)); });
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });

const clickMap = async (name, points) => {
  const box = await page.getByRole("application", { name }).boundingBox();
  for (const [x, y] of points) { await page.mouse.click(box.x + box.width * x, box.y + box.height * y); await page.waitForTimeout(300); }
};
await page.goto("http://localhost:3000/pricing/fees/simulate", { waitUntil: "networkidle" });
await page.waitForTimeout(2500);
await clickMap("Map: click to place the pickup, then the drop-off", [[0.45, 0.5], [0.6, 0.4]]);
await page.getByRole("button", { name: "Simulate" }).click();
await page.waitForTimeout(2500);
await page.screenshot({ path: `${out}/pricing-simulate-${width}.png`, fullPage: true });

await page.goto("http://localhost:3000/pricing/fees/history", { waitUntil: "networkidle" });
await page.waitForTimeout(2000);
await page.screenshot({ path: `${out}/pricing-history-${width}.png`, fullPage: true });
await page.goto("http://localhost:3000/pricing/fees/history?tab=as-of", { waitUntil: "networkidle" });
await page.waitForTimeout(2500);
await clickMap("Map: click to place the point, then an optional drop-off", [[0.45, 0.5]]);
await page.waitForTimeout(2000);
await page.screenshot({ path: `${out}/pricing-asof-${width}.png`, fullPage: true });
await page.goto("http://localhost:3000/pricing/fees/history?tab=export", { waitUntil: "networkidle" });
await page.waitForTimeout(1000);
await page.screenshot({ path: `${out}/pricing-export-${width}.png` });
await browser.close();
console.log("done");
