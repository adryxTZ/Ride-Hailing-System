// Driver sign-up and staff review against the real API:
// new driver → profile → vehicle → 6 documents (signed upload) → submit → cannot go online →
// staff lists pending → views a document → rejects one → approves → driver goes online →
// suspend takes them offline and blocks going online → reinstate →
// the licence expires (staff date fix) → document hold, cannot go online → the driver
// uploads a renewal → staff approve it → the hold lifts.
// Run: node tools/e2e-onboarding.mjs  (API on :8080 with NEXA_OTP_DEV_ECHO=true)
const host = "http://localhost:8080", api = `${host}/v1`, j = { "Content-Type": "application/json" };
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
async function call(method, path, token, body, expect) {
  const r = await fetch(api + path, { method, headers: { ...j, ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) throw new Error(`${method} ${path} → ${r.status} ${text.slice(0, 220)}`);
  return text ? JSON.parse(text) : null;
}
async function signIn(phone, app) {
  const otp = await call("POST", "/auth/otp/request", null, { phone, app });
  const t = await call("POST", "/auth/otp/verify", null, { phone, app, code: otp.dev_code ?? "0000" });
  return t.access_token;
}
const phone = `+2557${String(Date.now()).slice(-8)}`;
const dt = await signIn(phone, "driver"); log(`new driver ${phone}`);
let ob = await call("GET", "/driver/onboarding", dt);
log(`status ${ob.profile.status}, missing: ${ob.missing.join(", ")}`);
ob = await call("PUT", "/driver/onboarding/profile", dt, { full_name: "Juma Hassan", licence_number: "4455667001", licence_class: "B", national_id: "19900101-11111-00001-11" });
ob = await call("PUT", "/driver/onboarding/vehicle", dt, { service: "taxi", vehicle_class: "standard", plate: `t ${Date.now() % 1000} xyz`, make: "Toyota", model: "IST", colour: "Silver", year: 2012 });
log(`vehicle ${ob.vehicle.plate} (${ob.vehicle.status})`);
const jpeg = Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0, 16, 0x4a, 0x46, 0x49, 0x46, 0xff, 0xd9]);
log(`asked for: ${ob.requirements.map((r) => `${r.kind}${r.mandatory ? "" : " (optional)"}`).join(", ")}`);
const upload = async () => {
  const t = await call("POST", "/driver/onboarding/uploads", dt, { content_type: "image/jpeg" }, 201);
  const put = await fetch(t.upload_url.startsWith("/") ? host + t.upload_url : t.upload_url, { method: "PUT", headers: { "Content-Type": "image/jpeg" }, body: jpeg });
  if (put.status !== 204) throw new Error(`upload → ${put.status}`);
  return t.file_key;
};
const noDate = await call("POST", "/driver/onboarding/documents", dt, { kind: "LICENCE", file_key: await upload(), content_type: "image/jpeg" }, 400);
log(`licence without an expiry date → ${noDate.error.details.fields[0].code}`);
for (const r of ob.requirements.filter((x) => x.mandatory)) {
  const t = await call("POST", "/driver/onboarding/uploads", dt, { content_type: "image/jpeg" }, 201);
  const url = t.upload_url.startsWith("/") ? host + t.upload_url : t.upload_url;
  const put = await fetch(url, { method: "PUT", headers: { "Content-Type": "image/jpeg" }, body: jpeg });
  if (put.status !== 204) throw new Error(`upload ${r.kind} → ${put.status}`);
  ob = await call("POST", "/driver/onboarding/documents", dt, { kind: r.kind, file_key: t.file_key, content_type: "image/jpeg", expires_on: r.tracks_expiry ? "2029-05-01" : undefined });
}
log(`documents uploaded; can submit: ${ob.can_submit}`);
// Someone else's file cannot be attached.
await call("POST", "/driver/onboarding/documents", dt, { kind: "PHOTO", file_key: "documents/00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111.jpg", content_type: "image/jpeg" }, 400);
ob = await call("POST", "/driver/onboarding/submit", dt);
log(`submitted → ${ob.profile.status}`);
await call("PUT", "/driver/onboarding/profile", dt, { full_name: "Changed", licence_number: "x", national_id: "y" }, 409);
const blocked = await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.77, lng: 39.23 } }, 403);
log(`going online before approval → ${blocked.error.code}`);

const st = await signIn("+255626512332", "admin"); log("staff signed in");
const page = await call("GET", "/admin/drivers?status=PENDING_REVIEW", st);
const row = page.data.find((d) => d.phone === phone.replace("+", "+"));
if (!row) throw new Error("driver not in the pending list");
log(`pending list: ${page.total} (counts ${JSON.stringify(page.counts)})`);
let detail = await call("GET", `/admin/drivers/${row.user_id}`, st);
const doc = detail.documents[0];
const view = await fetch(doc.url.startsWith("/") ? host + doc.url : doc.url);
log(`staff opens ${doc.kind}: ${view.status} ${view.headers.get("content-type")}`);
detail = await call("POST", `/admin/drivers/${row.user_id}/documents/${detail.documents[1].id}/review`, st, { approve: false, reason: "Photo is blurry" });
log(`rejected ${detail.documents[1].kind}; driver missing: ${detail.missing.join(", ")}`);
await call("POST", `/admin/drivers/${row.user_id}/approve`, st, null, 409); // a rejected document blocks approval
await call("POST", `/admin/drivers/${row.user_id}/reject`, st, { reason: "Please re-upload your licence" });
ob = await call("GET", "/driver/onboarding", dt);
log(`driver sees ${ob.profile.status}: "${ob.profile.status_reason}", can edit: ${ob.can_edit}`);
const t2 = await call("POST", "/driver/onboarding/uploads", dt, { content_type: "image/jpeg" }, 201);
await fetch(host + t2.upload_url, { method: "PUT", headers: { "Content-Type": "image/jpeg" }, body: jpeg });
await call("POST", "/driver/onboarding/documents", dt, { kind: detail.documents[1].kind, file_key: t2.file_key, content_type: "image/jpeg", expires_on: "2029-05-01" });
await call("POST", "/driver/onboarding/submit", dt);
detail = await call("POST", `/admin/drivers/${row.user_id}/approve`, st);
log(`approved → ${detail.profile.status}, vehicle ${detail.vehicle.status}, history: ${detail.history.map((h) => h.action).join(" ← ")}`);
const on = await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.77, lng: 39.23 }, services: ["water"], vehicle_classes: ["water_5k"] });
log(`online with ${on.presence.services} / ${on.presence.classes} (the app's claim of water was ignored)`);
await call("POST", `/admin/drivers/${row.user_id}/suspend`, st, { reason: "Complaint under investigation" });
const after = await call("GET", "/driver/status", dt);
const again = await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.77, lng: 39.23 } }, 403);
log(`suspended: online=${after.online}, going online → ${again.error.code}`);
await call("POST", `/admin/drivers/${row.user_id}/reinstate`, st, { reason: "Cleared" });
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.77, lng: 39.23 } });
await call("PUT", "/driver/status", dt, { online: false });
await call("POST", `/admin/drivers/${row.user_id}/suspend`, st, { reason: "" }, 400);

// Expiry: staff record that the licence expired yesterday → the driver is held.
let docs = await call("GET", "/driver/documents", dt);
const lic = docs.documents.find((d) => d.kind === "LICENCE");
const yesterday = new Date(Date.now() - 86400000).toISOString().slice(0, 10);
await call("PATCH", `/admin/drivers/${row.user_id}/documents/${lic.id}/expiry`, st, { expires_on: yesterday, reason: "e2e: licence expired" });
docs = await call("GET", "/driver/documents", dt);
const held = await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.77, lng: 39.23 } }, 409);
log(`licence ${docs.documents.find((d) => d.kind === "LICENCE").expiry_state} → going online: ${held.error.code}`);
// The driver uploads the renewed licence; it waits for review beside the old one.
docs = await call("POST", "/driver/documents/renewals", dt, { kind: "LICENCE", file_key: await upload(), content_type: "image/jpeg", expires_on: "2031-01-31" }, 201);
const pending = docs.documents.find((d) => d.kind === "LICENCE").renewal;
log(`renewal ${pending.status} (expires ${pending.expires_on})`);
detail = await call("POST", `/admin/drivers/${row.user_id}/documents/${pending.id}/review`, st, { approve: true });
docs = await call("GET", "/driver/documents", dt);
const now = docs.documents.find((d) => d.kind === "LICENCE");
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.77, lng: 39.23 } });
log(`renewal approved: licence ${now.expiry_state} until ${now.expires_on}; the driver can go online again`);
await call("PUT", "/driver/status", dt, { online: false });
log("ONBOARDING E2E PASSED");
