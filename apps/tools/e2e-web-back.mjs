// Back with no history: a screen opened directly (link or refresh) must still go
// somewhere when its back button is pressed, with no "GO_BACK was not handled" error.
// Run: node tools/e2e-web-back.mjs  (rider web on :8083)
import { chromium } from "playwright";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 420, height: 820 } });
const errors = [];
page.on("console", (m) => { if (m.type() === "error" || m.text().includes("GO_BACK")) errors.push(m.text().slice(0, 160)); });
await page.goto("http://localhost:8083/", { waitUntil: "networkidle", timeout: 240000 });
await page.getByPlaceholder("07XX XXX XXX").fill("255626512332");
await page.getByText("Send code", { exact: true }).click();
await page.getByText("Verify", { exact: true }).click();
await page.getByText("Where to?", { exact: true }).waitFor({ timeout: 20000 });
let failed = 0;
for (const [path, title] of [["/account/settings", "Settings"], ["/account/profile", "Profile"], ["/account/places", "Saved places"]]) {
  errors.length = 0;
  await page.goto(`http://localhost:8083${path}`, { waitUntil: "networkidle", timeout: 240000 }); // fresh load: no history
  await page.getByText(title, { exact: true }).first().waitFor({ timeout: 20000 });
  await page.getByRole("button", { name: /back/i }).first().click();
  await page.waitForTimeout(1200);
  const at = new URL(page.url()).pathname;
  const bad = errors.filter((e) => e.includes("GO_BACK"));
  log(`${path} → back → ${at}${bad.length ? " (GO_BACK error)" : ""}`);
  if (at === path || bad.length) failed++;
}
await browser.close();
if (failed) throw new Error(`${failed} screen(s) could not go back`);
log("BACK E2E PASSED");
