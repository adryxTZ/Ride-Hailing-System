// Legal documents against the real API (spec DOC/specs/2026-10-24-legal-documents.md):
// 1) a rider accepts the current consent version;
// 2) staff fill a Legal detail, edit the rider Privacy Notice as a draft (with hostile
//    Markdown in it) and publish it with a reason;
// 3) the rider must accept again (consent version moved on, booking refused), accepts;
// 4) the public page renders with the placeholders filled and no script, link or
//    markup from the text; ETag gives 304; the JSON variant answers.
// Run: API=http://localhost:8091/v1 node tools/e2e-legal.mjs  (STAFF_EMAIL, STAFF_PASSWORD set)
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = process.env.API ?? "http://localhost:8080/v1";
const pass = (m) => console.log(`PASS ${m}`);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(`FAIL ${m}`); };
async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, { method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 300)}`);
  return text ? JSON.parse(text) : null;
}
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const CACHE_WAIT_MS = 500, CACHE_TRIES = 20; // the API reloads published texts within ~5 s

const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const phone = `+2556${String(Date.now()).slice(-8)}`;
const otp = await call("POST", "/auth/otp/request", null, { phone, app: "rider" });
const rider = (await call("POST", "/auth/otp/verify", null, { phone, app: "rider", code: otp.dev_code ?? "0000" })).access_token;

// 1) accept what is current
const before = await call("GET", "/legal/consent?app=rider", null);
if (!before.links?.some((l) => l.kind === "TERMS")) fail("the consent document must link the Terms");
await call("POST", "/me/consents", rider, { app: "RIDER", version: before.version }, 204);
if ((await call("GET", "/me", rider)).consent.required) fail("consent still required after accepting");
pass(`rider accepted consent v${before.version} (links: ${before.links.map((l) => l.kind).join(", ")})`);

// 2) details, draft, publish
const overview = await call("GET", "/admin/legal", staff);
const fields = { ...overview.details.fields, dpo_email: "dpo@e2e.example", registered_address: "Plot 7, E2E Road" };
await call("PUT", "/admin/legal/details", staff, { fields, reason: "e2e: legal details" });
pass("legal details saved (dpo_email, registered_address)");
const { versions } = await call("GET", "/admin/legal/rider/privacy-notice", staff);
const live = versions.find((v) => v.status === "PUBLISHED");
const marker = `E2E change ${Date.now()}`;
const hostile = `\n\n## ${marker}\n\nWrite to {{dpo_email}} at {{registered_address}}.\n\n<script>alert(1)</script> [click](javascript:alert(1)) <img src=x onerror=alert(1)>\n`;
const draft = await call("PUT", "/admin/legal/rider/privacy-notice/draft", staff,
  { title: live.title, summary: live.summary, body: { ...live.body, en: live.body.en + hostile } });
if (draft.status !== "DRAFT" || draft.version !== live.version + 1) fail(`draft v${draft.version} ${draft.status}`);
await call("POST", "/admin/legal/rider/privacy-notice/publish", staff, { reason: "" }, 400);
const published = await call("POST", "/admin/legal/rider/privacy-notice/publish", staff, { reason: "e2e: reviewed wording" });
if (published.status !== "PUBLISHED") fail("not published");
pass(`privacy notice v${published.version} published (a publish without a reason is refused)`);

// 3) asked again
let after;
for (let i = 0; i < CACHE_TRIES && !(after?.version > before.version); i++) {
  after = await call("GET", "/legal/consent?app=rider", null);
  if (after.version <= before.version) await sleep(CACHE_WAIT_MS);
}
if (!(after.version > before.version)) fail("the consent version did not move on");
const me = await call("GET", "/me", rider);
if (!me.consent.required || me.consent.accepted_version !== before.version) fail(`consent not required again: ${JSON.stringify(me.consent)}`);
const q = await call("POST", "/quotes", rider, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const cls = q.options.find((o) => o.available)?.vehicle_class ?? q.options[0].vehicle_class;
const refused = await call("POST", "/trips", rider, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 403);
if (refused.error.code !== "CONSENT_REQUIRED") fail(`booking answered ${refused.error.code}`);
pass(`consent v${after.version} required again; booking refused with CONSENT_REQUIRED`);
await call("POST", "/me/consents", rider, { app: "RIDER", version: before.version }, 409);
await call("POST", "/me/consents", rider, { app: "RIDER", version: after.version }, 204);
if ((await call("GET", "/me", rider)).consent.required) fail("still required after accepting the new version");
pass(`rider accepted v${after.version} (the old version is refused as stale)`);

// 4) the public page
// The page is cached in-process for a few seconds after a publish: retry briefly.
let page, html;
for (let i = 0; i < 10; i++) {
  page = await fetch(after.url, { cache: "no-store" });
  html = await page.text();
  if (html.includes(marker)) break;
  await sleep(1000);
}
if (page.status !== 200 || !page.headers.get("content-type")?.startsWith("text/html")) fail(`page ${page.status}`);
for (const want of [marker, "dpo@e2e.example", "Plot 7, E2E Road", `Version ${published.version}`, "&lt;script&gt;"]) {
  if (!html.includes(want)) fail(`page lacks ${want}`);
}
for (const bad of ["<script", "{{", "href=\"javascript", "<img"]) if (html.includes(bad)) fail(`page contains ${bad}`);
if (!page.headers.get("content-security-policy")?.includes("default-src 'none'")) fail("no CSP");
pass("public page renders with placeholders filled, no script, image or javascript link, CSP set");
const again = await fetch(after.url, { headers: { "If-None-Match": page.headers.get("etag") } });
if (again.status !== 304) fail(`ETag revalidation → ${again.status}`);
const json = await call("GET", `/legal/rider/privacy-notice?lang=en&format=json`, null);
if (json.version !== published.version || json.html.includes("<script")) fail("JSON variant");
const sw = await fetch(after.links.find((l) => l.kind === "TERMS").url.replace("lang=en", "lang=sw"));
if (!(await sw.text()).includes('lang="sw"')) fail("Swahili terms page");
pass("ETag gives 304; JSON variant and the Swahili Terms page answer");
console.log("LEGAL E2E PASSED");
