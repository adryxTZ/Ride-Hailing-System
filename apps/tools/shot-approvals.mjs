// Screenshots the approvals feature: the workflow list and editor, a pending
// refund request as the approver sees it, the inbox, and a person page's
// dashboard roles card. It turns the refund workflow on, holds one refund on
// the latest completed trip, and turns the workflow off at the end (the held
// request stays pending until someone decides or it expires).
// Run: STAFF_EMAIL=… STAFF_PASSWORD=… API_URL=http://localhost:8092/v1 ADMIN_URL=http://localhost:3022 node tools/shot-approvals.mjs
import { randomBytes } from "node:crypto";
import { chromium } from "playwright";

if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD");
const api = process.env.API_URL ?? "http://localhost:8080/v1";
const web = process.env.ADMIN_URL ?? "http://localhost:3000";
const out = process.env.OUT ?? "./tools/shots";
const APPROVER = { phone: "+255700440044", email: "e2e-approver@example.com", password: randomBytes(9).toString("base64url") };

async function call(method, path, token, body, headers = {}) {
  const r = await fetch(api + path, { method, headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}`, ...headers },
    body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (!r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
}
const login = async (email, password) =>
  (await (await fetch(`${api}/auth/staff/login`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, password }) })).json()).access_token;
const workflow = (token, enabled) => call("PUT", "/admin/approvals/workflows/trips.refund", token, {
  enabled, reason: enabled ? "Large refunds need Finance" : "Screenshots done",
  definition: { conditions: { amount_min: "1000", countries: ["TZ"] }, expire_hours: 48,
    steps: [{ name: "Finance check", mode: "ANY", roles: ["Finance"], approvals: 1 }, { name: "Operations sign-off", mode: "ALL", roles: ["Operations"], approvals: 1 }] } });

const admin = await login(process.env.STAFF_EMAIL, process.env.STAFF_PASSWORD);
await call("PUT", "/admin/staff", admin, { phone: APPROVER.phone, full_name: "E2E Finance Approver", roles: ["Finance"], email: APPROVER.email, password: APPROVER.password });
await workflow(admin, true);
const trips = await call("GET", "/admin/trips?status=COMPLETED&limit=20", admin);
let held;
for (const t of trips.data ?? []) {
  const r = await call("GET", `/admin/trips/${t.id}/refunds`, admin).catch(() => null);
  if (!r || Number(r.refundable) < 1000) continue;
  const res = await fetch(`${api}/admin/trips/${t.id}/refunds`, { method: "POST", headers: { "Content-Type": "application/json", Authorization: `Bearer ${admin}` },
    body: JSON.stringify({ amount: "1000", borne_by: "DRIVER", reason: "Rider was charged for a long detour" }) });
  if (res.status === 202) { held = (await res.json()).approval_request; break; }
}
console.log("held request:", held?.id);

const browser = await chromium.launch();
const shoot = async (email, password, pages) => {
  const page = await (await browser.newContext({ viewport: { width: 1600, height: 1000 } })).newPage();
  page.on("pageerror", (e) => console.log("pageerror:", String(e).slice(0, 200)));
  await page.goto(`${web}/login`, { waitUntil: "networkidle", timeout: 180000 });
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
  for (const [name, path] of pages) {
    await page.goto(web + path, { waitUntil: "networkidle", timeout: 120000 });
    await page.waitForTimeout(1800);
    await page.screenshot({ path: `${out}/approvals-${name}.png`, fullPage: true });
  }
  return page;
};
const adminPage = await shoot(process.env.STAFF_EMAIL, process.env.STAFF_PASSWORD, [
  ["workflows", "/settings/approvals"],
  ["editor", "/settings/approvals/trips.refund"],
  ["inbox-mine", "/approvals?box=mine"],
  ...(held ? [["request-requester", `/approvals/${held.id}`]] : []),
  ["staff", "/staff"],
]);
const riders = await call("GET", "/admin/riders?limit=1", admin).catch(() => ({ data: [] }));
if (riders.data?.[0]) {
  await adminPage.goto(`${web}/riders/${riders.data[0].id}`, { waitUntil: "networkidle" });
  await adminPage.waitForTimeout(2500);
  const card = adminPage.locator("section", { hasText: "Dashboard roles" }).last();
  await card.getByRole("button", { name: /dashboard roles|change roles/i }).click();
  await card.screenshot({ path: `${out}/approvals-rider-roles.png` });
}

// The shared 202 helper in the UI: with the staff-roles workflow on, a role
// edit from the Staff list shows "Sent for approval" instead of failing.
const rolesFlow = (enabled) => call("PUT", "/admin/approvals/workflows/staff.roles", admin, { enabled,
  reason: enabled ? "Role changes need a second Super Admin" : "Screenshots done",
  definition: { conditions: {}, expire_hours: 24, steps: [{ name: "Second admin", mode: "ANY", roles: ["Super Admin"], approvals: 1 }] } });
await rolesFlow(true);
await adminPage.goto(`${web}/staff`, { waitUntil: "networkidle" });
await adminPage.waitForTimeout(1200);
await adminPage.getByRole("row", { name: /E2E Finance Approver/ }).getByRole("button", { name: "Edit roles" }).click();
if (!adminPage.url().endsWith("/staff")) throw new Error("Edit roles on a row must not open the person's page");
await adminPage.getByRole("button", { name: "Support", exact: true }).click();
await adminPage.waitForTimeout(900); // let the card's entrance finish
await adminPage.screenshot({ path: `${out}/approvals-staff-edit.png` });
await adminPage.getByRole("button", { name: "Save roles" }).click();
await adminPage.getByPlaceholder("Why are you doing this?").fill("Covers support shifts this month");
await adminPage.getByRole("dialog").getByRole("button", { name: "Save" }).click();
await adminPage.getByText("Sent for approval").waitFor({ timeout: 10000 });
await adminPage.waitForTimeout(400);
await adminPage.screenshot({ path: `${out}/approvals-sent-notice.png` });
const mine = await call("GET", "/admin/approvals/requests?box=mine", admin);
for (const r of mine.data.filter((x) => x.action === "staff.roles" && x.status === "PENDING"))
  await call("POST", `/admin/approvals/requests/${r.id}/cancel`, admin, { note: "screenshot only" });
await rolesFlow(false);
await shoot(APPROVER.email, APPROVER.password, [
  ["inbox", "/approvals"],
  ...(held ? [["request-approver", `/approvals/${held.id}`]] : []),
]);
await browser.close();
await workflow(admin, false);
console.log("done");
