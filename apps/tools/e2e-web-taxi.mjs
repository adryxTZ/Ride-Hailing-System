// Smoke test: taxi journey on the web build, following the taxi story video —
// sign in → taxi home → Route (saved place) → ride options → confirm →
// "Looking for a driver" → "Ride requested" → cancel ("Are you sure?" → reason) → back on options with "Ride cancelled".
// Run: OUT=./tools/shots node tools/e2e-web-taxi.mjs   (needs the rider web build on :8083)
import { chromium } from "playwright";
const out = process.env.OUT ?? ".";
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 412, height: 915 }, deviceScaleFactor: 2 });
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
// Screens under the top of the stack stay mounted (hidden); only match what is visible.
const vis = (t) => page.locator(`text="${t}" >> visible=true`).first();
const shot = (name) => page.screenshot({ path: `${out}/${name}.png` });
page.on("console", (m) => { if (m.type() === "error") log("console.error: " + m.text().slice(0, 160)); });
await page.context().grantPermissions(["geolocation"]);
await page.context().setGeolocation({ latitude: -6.7764, longitude: 39.229 });
await page.goto("http://localhost:8083/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255626512332");
await vis("Send code").click();
await vis("Verify").click();
await page.getByText(/^(Go wherever, whenever\.|Great to see you\.)$/).waitFor({ timeout: 20000 });
await page.waitForTimeout(1200); await shot("t0-taxi-home"); log("taxi home");
await vis("Where to?").click();
await vis("Route").waitFor({ timeout: 20000 });
await vis("Gym").waitFor({ timeout: 20000 });
await shot("t1-route"); log("route list");
await vis("Gym").click();
await vis("Confirm order").waitFor({ timeout: 20000 });
await page.waitForTimeout(2500); await shot("t2-options");
log("options: " + (await page.locator("text=/TZS [0-9,]+/").count()) + " priced rows");
await vis("Confirm order").click(); log("confirmed");
await vis("Looking for a driver").waitFor({ timeout: 20000 });
await shot("t3-looking"); log("looking for a driver");
await vis("Ride requested").waitFor({ timeout: 20000 });
await page.waitForTimeout(800); await shot("t4-requested"); log("ride requested");
await vis("Cancel ride").click();
await vis("Are you sure?").waitFor({ timeout: 20000 });
await page.waitForTimeout(500); await shot("t5-are-you-sure");
await page.locator('text="Cancel ride" >> visible=true').last().click();
await vis("What went wrong?").waitFor({ timeout: 20000 });
await page.getByText(/Pickup is taking too long|Kuchukuliwa/).first().click();
await shot("t6-reason");
await vis("Done").click(); log("cancelled with reason");
await vis("Ride cancelled").waitFor({ timeout: 20000 });
await page.waitForTimeout(2000); await shot("t7-cancelled"); log("back on options with Ride cancelled");
log("TAXI E2E PASSED");
await browser.close();
