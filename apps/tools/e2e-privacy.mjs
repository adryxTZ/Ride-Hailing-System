// Data protection against the real API (spec 2026-10-23-compliance-trust.md, S4):
// 1) a new rider is told consent is required; booking is refused (403) until they accept;
// 2) they ask for a copy of their data; staff approve; the export becomes READY and the
//    downloaded JSON has every section and their own phone only;
// 3) they ask for deletion; while a ride is open it is blocked (ACTIVE_TRIP); after
//    cancelling, staff approve and the account is anonymised; the phone can sign up again.
// Run: node tools/e2e-privacy.mjs  (API on :8080 with workers)
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(m); };
async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, { method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 300)}`);
  return text ? JSON.parse(text) : null;
}
const signIn = async (phone) => {
  const otp = await call("POST", "/auth/otp/request", null, { phone, app: "rider" }); // dev echo returns the code for new numbers
  return call("POST", "/auth/otp/verify", null, { phone, app: "rider", code: otp.dev_code ?? "0000" });
};
const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const phone = `+2556${String(Date.now()).slice(-8)}`;
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };

// 1) consent
const pair = await signIn(phone);
if (!pair.consent_required) fail("a new rider must be asked for consent");
const rt = pair.access_token;
const me = await call("GET", "/me", rt);
const userId = me.user_id;
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const cls = q.options.find((o) => o.available).vehicle_class;
const refused = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 403);
log(`booking before consent → ${refused.error.code}`);
const doc = await call("GET", "/legal/consent?app=rider", null);
const stale = await call("POST", "/me/consents", rt, { app: "RIDER", version: doc.version + 1 }, 409);
log(`stale version → ${stale.error.code} (current ${stale.error.details?.current_version})`);
await call("POST", "/me/consents", rt, { app: "RIDER", version: doc.version }, 204);
if ((await call("GET", "/me", rt)).consent.required) fail("consent still required after accepting");
log(`accepted "${doc.title}" v${doc.version}`);

// 2) export
const exp = await call("POST", "/me/privacy-requests", rt, { kind: "EXPORT" }, 201);
const dup = await call("POST", "/me/privacy-requests", rt, { kind: "EXPORT" }, 409);
log(`export requested (due ${exp.due_at.slice(0, 10)}); a second one → ${dup.error.code}`);
await call("POST", `/admin/privacy/requests/${exp.id}/approve`, staff, { reason: "e2e: identity checked" });
let ready;
for (let i = 0; i < 40 && !ready; i++) {
  const mine = (await call("GET", "/me/privacy-requests", rt)).data.find((r) => r.id === exp.id);
  if (mine.status === "READY") ready = mine;
  else await sleep(500);
}
if (!ready) fail("the export never became READY");
const link = await call("GET", `/me/privacy-requests/${exp.id}/download`, rt);
const file = await (await fetch(new URL(link.url, api))).json(); // the local store signs relative URLs
const sections = Object.keys(file.data).sort();
log(`export READY: ${sections.length} sections (${sections.join(", ")})`);
if (file.data.profile.phone !== phone) fail("the export must be this person's");
for (const s of ["profile", "consents", "trips", "payments", "receipts", "messages_sent"]) if (!(s in file.data)) fail(`export missing ${s}`);

// 3) delete: blocked during a ride, done after
const trip = await call("POST", "/trips", rt, { quote_id: (await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF })).quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
const del = await call("POST", "/me/privacy-requests", rt, { kind: "DELETE" }, 201);
const blocked = await call("POST", `/admin/privacy/requests/${del.id}/approve`, staff, { reason: "e2e" }, 409);
log(`delete during a ride → ${blocked.error.code} ${JSON.stringify(blocked.error.details?.blockers)}`);
if (!blocked.error.details?.blockers?.includes("ACTIVE_TRIP")) fail("an open ride must block deletion");
await call("POST", `/trips/${trip.id}/cancel`, rt, { reason: "other" });
const done = await call("POST", `/admin/privacy/requests/${del.id}/approve`, staff, { reason: "e2e: asked in app" });
if (done.status !== "DONE") fail("delete not done");
const gone = await fetch(`${api}/me`, { headers: { Authorization: `Bearer ${rt}` } });
log(`deleted; the old session → ${gone.status}`);
await sleep(61_000); // the OTP resend limit per phone is a minute
const again = await signIn(phone);
const newMe = await call("GET", "/me", again.access_token);
if (newMe.user_id === userId) fail("the phone must sign up as a new account");
log("the phone signed up again as a new account");
log("PRIVACY E2E PASSED");
