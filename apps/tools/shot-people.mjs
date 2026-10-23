// Screenshots the staff driver and rider pages (details left, trips right), with
// a search in the trips panel. Run: node tools/shot-people.mjs <driverId> <riderId> [<driverId>…]
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const [, , ...ids] = process.argv;
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1600, height: 1000 } })).newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 200)); });
page.on("pageerror", (e) => console.log("pageerror:", String(e).slice(0, 200)));
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
for (const spec of ids) {
  const [kind, id] = spec.split(":");
  await page.goto(`http://localhost:3000/${kind}s/${id}`, { waitUntil: "networkidle", timeout: 180000 });
  await page.waitForTimeout(2500);
  await page.screenshot({ path: `${out}/person-${kind}-${id.slice(0, 8)}.png` });
  await page.screenshot({ path: `${out}/person-${kind}-${id.slice(0, 8)}-full.png`, fullPage: true });
  const search = page.getByLabel(/Code, (rider|driver) or address/);
  await search.fill("Gym");
  await page.waitForTimeout(1500);
  const found = await page.locator("aside").getByText(/found/).textContent();
  console.log(kind, id.slice(0, 8), "search Gym:", found);
}
await browser.close();
