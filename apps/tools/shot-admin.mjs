// Screenshots of the admin dashboard pages, signed in as the Super Admin account (email + password).
// Run: node tools/shot-admin.mjs [path ...]   (admin on :3000, API on :8080). Add THEME=dark for dark mode.
import { chromium } from "playwright";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");
const out = process.env.OUT ?? "./tools/shots";
const theme = process.env.THEME ?? "light";
const paths = process.argv.slice(2).length ? process.argv.slice(2) : ["/"];
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: theme });
const page = await ctx.newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("console.error:", m.text().slice(0, 200)); });
await page.goto("http://localhost:3000/login", { waitUntil: "networkidle", timeout: 180000 });
await page.screenshot({ path: `${out}/admin-login.png` });
await page.getByLabel("Email").fill(process.env.STAFF_EMAIL);
await page.getByLabel("Password").fill(process.env.STAFF_PASSWORD);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
for (const p of paths) {
  await page.goto(`http://localhost:3000${p}`, { waitUntil: "networkidle", timeout: 180000 });
  await page.waitForTimeout(2200);
  const name = p === "/" ? "overview" : p.replace(/^\//, "").replace(/[/?=&]/g, "-");
  await page.screenshot({ path: `${out}/admin-${name}${theme === "dark" ? "-dark" : ""}.png`, fullPage: true });
  console.log("shot", p);
}
await browser.close();
