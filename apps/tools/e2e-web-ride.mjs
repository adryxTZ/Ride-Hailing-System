// Full ride across both web apps against the real API: driver online → rider books →
// driver sees the offer and accepts → rider sees "Pickup in …" → arrived → trip →
// payment → completed, with screenshots of both apps at each step.
// Run: OUT=./tools/shots node tools/e2e-web-ride.mjs  (rider web on :8083, driver web on :8084)
import { chromium } from "playwright";
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const browser = await chromium.launch();
// Clear a booking left by an interrupted run (one active trip per rider).
{
  const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
  await fetch(`${api}/auth/otp/request`, { method: "POST", headers: j, body: JSON.stringify({ phone: "+255626512332", app: "rider" }) });
  const { access_token } = await (await fetch(`${api}/auth/otp/verify`, { method: "POST", headers: j, body: JSON.stringify({ phone: "+255626512332", code: "0000", app: "rider" }) })).json();
  const auth = { ...j, Authorization: `Bearer ${access_token}` };
  const { trip } = await (await fetch(`${api}/trips/active`, { headers: auth })).json();
  if (trip) await fetch(`${api}/trips/${trip.id}/cancel`, { method: "POST", headers: auth, body: JSON.stringify({ reason: "other", note: "e2e cleanup" }) });
}
async function app(url, phone, geo, name) {
  const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: geo, permissions: ["geolocation"] });
  const page = await ctx.newPage();
  page.on("console", (m) => { if (m.type() === "error") log(`${name} console.error: ${m.text().slice(0, 140)}`); });
  await page.goto(url, { waitUntil: "load", timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
  await page.getByPlaceholder("07XX XXX XXX").fill(phone);
  await page.locator('text="Send code" >> visible=true').click();
  await page.locator('text="Verify" >> visible=true').click();
  const vis = (t) => page.locator(`text="${t}" >> visible=true`).first();
  const visRe = (re) => page.getByText(re).locator("visible=true").first();
  return { page, vis, visRe, shot: (n) => page.screenshot({ path: `${out}/${n}.png` }) };
}
const driver = await app("http://localhost:8084/", "255715123702", { latitude: -6.7770, longitude: 39.2300 }, "driver");
// Either offline (go online) or still online from an earlier run.
const state = await Promise.race([
  driver.vis("Go online").waitFor({ timeout: 60000 }).then(() => "offline"),
  driver.vis("Finding trips near you").waitFor({ timeout: 60000 }).then(() => "online"),
]);
if (state === "offline") { await driver.page.waitForTimeout(600); await driver.vis("Go online").click(); }
await driver.vis("Finding trips near you").waitFor({ timeout: 30000 }).catch(async (e) => {
  await driver.shot("r0-driver-stuck");
  log("driver text: " + (await driver.page.innerText("body")).slice(0, 300).replace(/\n/g, " | "));
  throw e;
});
log("driver online");

const rider = await app("http://localhost:8083/", "255626512332", { latitude: -6.7764, longitude: 39.229 }, "rider");
await rider.vis("Where to?").click();
await rider.vis("Gym").waitFor({ timeout: 20000 }); await rider.vis("Gym").click();
await rider.vis("Confirm order").waitFor({ timeout: 20000 }); await rider.page.waitForTimeout(1500);
await rider.visRe(/^(Kawaida|Standard)$/).click(); // the test driver drives a standard car
await rider.vis("Confirm order").click(); log("rider confirmed");

await driver.visRe(/wants to ride with you$/).waitFor({ timeout: 20000 });
await driver.page.waitForTimeout(600); await driver.shot("r1-driver-offer"); log("driver got the offer");
await driver.vis("Accept").click(); log("driver accepted");
await driver.vis("Head to pickup").waitFor({ timeout: 20000 }); await driver.shot("r2-driver-to-pickup");
await rider.vis("Pickup in").waitFor({ timeout: 20000 }); await rider.page.waitForTimeout(800); await rider.shot("r3-rider-assigned"); log("rider sees the driver assigned");

// Chat through the screens: rider writes, driver reads and replies.
await rider.page.getByLabel("Message driver").click();
await rider.page.getByPlaceholder("Message").fill("I'm at the gate");
await rider.page.getByLabel("Send").click();
await rider.vis("I'm at the gate").waitFor({ timeout: 10000 }); await rider.shot("r3b-rider-chat"); log("rider sent a chat message");
await driver.page.getByLabel("Message rider").click();
await driver.vis("I'm at the gate").waitFor({ timeout: 15000 }); log("driver sees the message");
await driver.visRe(/^Nimefika$|^I've arrived$/).first().click().catch(async () => {
  await driver.page.getByPlaceholder("Message").fill("I've arrived"); await driver.page.getByLabel("Send").click();
});
await driver.page.waitForTimeout(800); await driver.shot("r3c-driver-chat");
await rider.visRe(/^(Nimefika|I've arrived)$/).waitFor({ timeout: 15000 }); log("rider got the reply live");
await driver.page.goBack(); await rider.page.goBack();
await driver.vis("I've arrived").waitFor({ timeout: 15000 });

const steps = [
  ["I've arrived", "Your driver has arrived", "r4-rider-arrived"],
  ["Start trip", "Arriving at", "r5-rider-riding"],
  ["Arrived at destination", "You have arrived", null],
  ["End trip", /^Pay TZS [0-9,]+$/, "r6-rider-pay"],
  [/^Cash collected · TZS/, "Thanks for riding", "r7-rider-done"],
];
for (const [button, riderText, shot] of steps) {
  if (button instanceof RegExp && String(button).includes("Cash")) { await driver.page.waitForTimeout(500); await driver.shot("r6b-driver-collect"); }
  await (button instanceof RegExp ? driver.visRe(button) : driver.vis(button)).click(); log(`driver: ${button}`);
  await (riderText instanceof RegExp ? rider.visRe(riderText) : rider.vis(riderText)).waitFor({ timeout: 20000 }); log(`rider: ${riderText}`);
  if (shot) { await rider.page.waitForTimeout(500); await rider.shot(shot); }
}
// Both rate: the rider gives 5 stars with a chip, the driver 4 stars.
await rider.page.getByLabel("5 stars").click(); await rider.visRe(/^Friendly$/).click();
await rider.vis("Submit rating").click();
await rider.visRe(/You rated this trip 5 of 5/).waitFor({ timeout: 15000 }); await rider.shot("r7b-rider-rated"); log("rider rated 5");
await driver.visRe(/^Rate /).waitFor({ timeout: 15000 }); await driver.shot("r7c-driver-rate");
await driver.page.getByLabel("4 stars").click(); await driver.vis("Submit rating").click(); log("driver rated 4");
await driver.vis("Finding trips near you").waitFor({ timeout: 20000 }); await driver.shot("r8-driver-free"); log("driver back to finding trips");
await rider.vis("Done").click(); await rider.vis("Where to?").waitFor({ timeout: 20000 }); log("rider back home");
// Afterwards: the trip is in the rider's Activity (with its receipt) and the driver's History / Earnings.
await rider.page.goto("http://localhost:8083/activity", { waitUntil: "load" });
await rider.vis("Rebook").waitFor({ timeout: 20000 }); await rider.page.waitForTimeout(1500); await rider.shot("r9-rider-activity");
await rider.page.locator("text=/^Gym$/ >> visible=true").first().click();
await rider.vis("Receipt").waitFor({ timeout: 20000 }); await rider.page.waitForTimeout(1200); await rider.shot("r10-rider-trip-detail");
await driver.page.goto("http://localhost:8084/history", { waitUntil: "load" });
await driver.visRe(/Completed/).waitFor({ timeout: 20000 }); await driver.shot("r11-driver-history");
await driver.page.goto("http://localhost:8084/earnings", { waitUntil: "load" });
await driver.vis("You earned today").waitFor({ timeout: 20000 }); await driver.page.waitForTimeout(500); await driver.shot("r12-driver-earnings");
log("activity, history and earnings show the trip");
log("RIDE E2E PASSED");
await browser.close();
