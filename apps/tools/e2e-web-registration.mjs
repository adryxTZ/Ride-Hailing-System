// Driver registration in the driver web app, as a new driver would do it on the phone:
// sign in with a new number → "Become a driver" → personal details → vehicle (ride type, plate…)
// → 6 photos (file picker) → Send for review → "We're checking your documents" → staff approve
// over the API → Home shows "Go online" with the registered vehicle.
// Run: OUT=./tools/shots node tools/e2e-web-registration.mjs  (driver web on :8084, API on :8080)
import { chromium } from "playwright";
import { fileURLToPath } from "node:url";
const api = "http://localhost:8080/v1", j = { "Content-Type": "application/json" };
const out = process.env.OUT ?? ".";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const photo = fileURLToPath(new URL("./fixtures/document.png", import.meta.url));
const local = `07${String(Date.now()).slice(-8)}`;

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 412, height: 915 }, geolocation: { latitude: -6.7764, longitude: 39.229 }, permissions: ["geolocation"] });
const page = await ctx.newPage();
const vis = (re) => page.getByText(re).locator("visible=true").first();
const shot = (n) => page.screenshot({ path: `${out}/${n}.png` });
process.on("uncaughtException", async (e) => { await shot("registration-fail").catch(() => {}); console.error(e.message.split("\n")[0]); process.exit(1); });

await page.goto("http://localhost:8084/", { waitUntil: "load", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").waitFor({ timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill(local);
await vis(/^Send code$/).click();
const code = (await vis(/Development: code is \d+/).textContent()).match(/\d+$/)[0];
await page.locator("input:visible").first().fill(code);
await vis(/^Verify$/).click(); log(`signed in as new driver ${local}`);

await vis(/^Become a driver$/).waitFor({ timeout: 30000 }); await page.waitForTimeout(800); await shot("reg-1-home");
await vis(/^Continue registration$/).click();
await vis(/^Registration$/).waitFor({ timeout: 20000 }); await page.waitForTimeout(800); await shot("reg-2-checklist");

await vis(/^Personal details$/).click();
const fields = page.locator("input:visible");
await fields.nth(0).fill("Juma Hassan");
await fields.nth(1).fill("4455667001");
await page.getByLabel("Class B").click();
await fields.nth(2).fill("2029-05-01");
await fields.nth(3).fill("19900101111110000111");
await shot("reg-3-profile");
await vis(/^Save$/).click(); log("personal details saved");

await vis(/^Vehicle$/).waitFor({ timeout: 20000 }); await vis(/^Vehicle$/).click();
await vis(/^(Standard|Kawaida)$/).click();
const vf = page.locator("input:visible");
await vf.nth(0).fill(`T ${String(Date.now()).slice(-3)} ABC`);
await vf.nth(1).fill("Toyota"); await vf.nth(2).fill("IST"); await vf.nth(3).fill("Silver"); await vf.nth(4).fill("2012");
await shot("reg-4-vehicle");
await vis(/^Save$/).click(); log("vehicle saved");

// The mandatory documents; licence and insurance then ask for their expiry date.
const DOCS = [["Your photo", "Picha yako"], ["Driving licence", "Leseni ya udereva", true], ["National ID", "Kitambulisho cha taifa"],
  ["Vehicle registration card", "Kadi ya usajili wa gari"], ["Insurance sticker", "Stika ya bima", true], ["Photo of the vehicle", "Picha ya gari"]];
const esc = (t) => t.replace(/[()]/g, "\\$&");
for (const [en, sw, expiry] of DOCS) {
  const row = vis(new RegExp(`^(${esc(en)}|${esc(sw)})$`));
  await row.scrollIntoViewIfNeeded();
  const chooser = page.waitForEvent("filechooser", { timeout: 15000 });
  await row.click();
  await (await chooser).setFiles(photo);
  if (expiry) {
    await page.getByPlaceholder("YYYY-MM-DD").last().waitFor({ timeout: 10000 });
    await page.getByPlaceholder("YYYY-MM-DD").last().fill("2029-05-01");
    if (en === "Driving licence") await shot("reg-4b-expiry");
    await vis(/^Save$/).click();
  }
  await page.waitForTimeout(1500); // upload + checklist refresh
}
log("6 photos uploaded");
await page.waitForTimeout(600); await shot("reg-5-complete");
await vis(/^Send for review$/).click();
await vis(/^We.re checking your documents$/).waitFor({ timeout: 20000 }); await page.waitForTimeout(800);
await shot("reg-6-submitted"); log("sent for review");

// Staff approve (the dashboard does this; here over the API).
const sign = async (phone, app) => {
  const otp = await (await fetch(`${api}/auth/otp/request`, { method: "POST", headers: j, body: JSON.stringify({ phone, app }) })).json();
  const t = await (await fetch(`${api}/auth/otp/verify`, { method: "POST", headers: j, body: JSON.stringify({ phone, app, code: otp.dev_code ?? "0000" }) })).json();
  return t.access_token;
};
const st = await sign("+255626512332", "admin");
const list = await (await fetch(`${api}/admin/drivers?status=PENDING_REVIEW&q=Juma`, { headers: { Authorization: `Bearer ${st}` } })).json();
const row = list.data[0];
const ok = await fetch(`${api}/admin/drivers/${row.user_id}/approve`, { method: "POST", headers: { Authorization: `Bearer ${st}` } });
if (!ok.ok) throw new Error(`approve → ${ok.status}`);
log("staff approved");

await page.goto("http://localhost:8084/", { waitUntil: "load" });
await vis(/^Go online$/).waitFor({ timeout: 30000 });
const plate = await vis(/^T\d{3}ABC$/).textContent();
await page.waitForTimeout(800); await shot("reg-7-approved-home"); log(`home shows Go online with ${plate}`);
log("REGISTRATION E2E PASSED");
await browser.close();
