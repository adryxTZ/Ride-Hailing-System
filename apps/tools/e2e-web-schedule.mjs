// Booking ahead (rider web): Home "Later" → pick tomorrow's first slot → route → options shows
// "Schedule · Tomorrow HH:MM" → "Your ride is scheduled" → Activity "Upcoming" → Cancel.
// Run: OUT=./tools/shots node tools/e2e-web-schedule.mjs  (rider web on :8083, API on :8080)
import { chromium } from "playwright";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const post = async (path, body, token) => (await fetch(api + path, { method: "POST", headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: JSON.stringify(body) })).json();
await post("/auth/otp/request", { phone: "+255626512332", app: "rider" });
const { access_token: rt } = await post("/auth/otp/verify", { phone: "+255626512332", code: "0000", app: "rider" });
const auth = { Authorization: `Bearer ${rt}` };
const trips = async () => (await (await fetch(`${api}/trips`, { headers: auth })).json()).data;
for (const t of await trips()) if (t.status === "SCHEDULED") await post(`/trips/${t.id}/cancel`, { reason: "plans_changed" }, rt);
const { trip: old } = await (await fetch(`${api}/trips/active`, { headers: auth })).json();
if (old) await post(`/trips/${old.id}/cancel`, { reason: "other" }, rt);

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.7764, longitude: 39.229 }, permissions: ["geolocation"] });
const page = await ctx.newPage();
page.on("framenavigated", (f) => { if (f === page.mainFrame()) log(`→ ${f.url()}`); });
page.on("response", (r) => { if (r.url().endsWith("/v1/trips") && r.request().method() === "POST") r.json().then((b) => log(`POST /trips → ${b.status ?? JSON.stringify(b).slice(0, 80)}`)).catch(() => {}); });
const vis = (re) => page.getByText(re).locator("visible=true").first();
const shot = (n) => page.screenshot({ path: `${out}/${n}.png` });
process.on("uncaughtException", async (e) => { await shot("schedule-fail").catch(() => {}); console.error(e.message.split("\n")[0]); process.exit(1); });
await page.goto("http://localhost:8083/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255626512332");
await vis(/^Send code$/).click(); await vis(/^Verify$/).click();
await vis(/^Where to\?$/).waitFor({ timeout: 60000 });

await vis(/^Later$/).click();
await vis(/^Schedule a ride$/).waitFor({ timeout: 15000 });
await vis(/^Tomorrow$/).click(); await page.waitForTimeout(600);
await vis(/^07:30$/).click(); await page.waitForTimeout(300);
await shot("schedule-1-picker"); log("picked tomorrow 07:30");
await vis(/^Set pickup time/).click();
await vis(/^Route$/).waitFor({ timeout: 15000 });
await vis(/^Gym$/).waitFor({ timeout: 20000 }); await vis(/^Gym$/).click();
await vis(/^Schedule · Tomorrow 07:30$/).waitFor({ timeout: 25000 }); await page.waitForTimeout(1500);
await shot("schedule-2-options"); log("options: Schedule · Tomorrow 07:30");
await vis(/^Schedule · Tomorrow 07:30$/).click();
await vis(/^Your ride is scheduled$/).waitFor({ timeout: 15000 }); await page.waitForTimeout(600);
await shot("schedule-3-confirmed"); log("ride scheduled");
const t = (await trips()).find((x) => x.status === "SCHEDULED");
log(`server: ${t.status} for ${t.scheduled_for}`);
await vis(/^View upcoming rides$/).click();
await vis(/^Upcoming$/).waitFor({ timeout: 15000 }); await page.waitForTimeout(800);
await shot("schedule-4-upcoming"); log("Activity shows Upcoming");
await page.getByLabel("Cancel scheduled ride").first().click();
await page.waitForTimeout(2500);
if ((await trips()).some((x) => x.status === "SCHEDULED")) throw new Error("still scheduled after cancel");
log("cancelled from Activity");
log("SCHEDULE E2E PASSED");
await browser.close();
