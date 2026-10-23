// "Who is riding?" (rider web): a pickup away from the phone asks whether the ride is for
// someone else; choosing "Someone else" needs a name and phone, which the booked trip carries
// and the driver sees. A pickup where the rider stands goes straight to the ride options.
// Run: OUT=./tools/shots node tools/e2e-web-details.mjs  (rider web on :8083, API on :8080)
import { chromium } from "playwright";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const post = async (path, body, token) => (await fetch(api + path, { method: "POST", headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: JSON.stringify(body) })).json();
await post("/auth/otp/request", { phone: "+255626512332", app: "rider" });
const { access_token: rt } = await post("/auth/otp/verify", { phone: "+255626512332", code: "0000", app: "rider" });
const auth = { Authorization: `Bearer ${rt}` };
const active = async () => (await (await fetch(`${api}/trips/active`, { headers: auth })).json()).trip;
const cancelActive = async () => { const t = await active(); if (t) await post(`/trips/${t.id}/cancel`, { reason: "other", note: "e2e" }, rt); };
await cancelActive();

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.7764, longitude: 39.229 }, permissions: ["geolocation"] });
const page = await ctx.newPage();
const vis = (re) => page.getByText(re).locator("visible=true").first();
process.on("uncaughtException", async (e) => { await page.screenshot({ path: `${out}/details-fail.png` }).catch(() => {}); console.error(e.message.split("\n")[0]); process.exit(1); });
await page.goto("http://localhost:8083/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255626512332");
await vis(/^Send code$/).click(); await vis(/^Verify$/).click();
await vis(/^Where to\?$/).waitFor({ timeout: 60000 });

// Pickup away from the phone → "Who is riding?"
await vis(/^Where to\?$/).click();
await vis(/^Route$/).waitFor({ timeout: 20000 }); await page.waitForTimeout(2500);
await vis(/Chui Street|Current location|Mtaa/).click();
await page.getByPlaceholder("Pickup location").fill("sinza");
await vis(/Sinza/).waitFor({ timeout: 20000 }); await vis(/Sinza/).click();
await page.getByPlaceholder("Dropoff location").fill("mlimani");
await vis(/^Mlimani City/).waitFor({ timeout: 20000 }); await vis(/^Mlimani City/).click();
await vis(/^Who is riding\?$/).waitFor({ timeout: 15000 });
await vis(/^Someone else$/).click(); await page.waitForTimeout(1200);
await page.getByPlaceholder("Name").fill("Neema Joseph");
await page.getByPlaceholder("07XX XXX XXX").fill("0754 111 222");
await page.screenshot({ path: `${out}/details-who-rides.png` });
await vis(/^Continue$/).click();
await vis(/^Confirm order$/).waitFor({ timeout: 25000 }); await page.waitForTimeout(2000);
await vis(/^Confirm order$/).click();
await page.getByText(/^(Looking for a driver|Ride requested)$/).locator("visible=true").first().waitFor({ timeout: 20000 });
const t = await active();
log(`booked for: ${JSON.stringify(t.contacts)}`);
if (t.contacts?.pickup?.name !== "Neema Joseph") throw new Error("passenger missing on the trip");
await cancelActive();
log("DETAILS E2E PASSED");
await browser.close();
