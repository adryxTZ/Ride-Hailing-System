// Approval workflows against the real API (spec 2026-10-24-approval-workflows):
// 1) a second staff member (Finance) is created through the API;
// 2) the refund workflow is turned on for amounts ≥ THRESHOLD, one Finance step;
// 3) a refund below the threshold applies at once (201);
// 4) a refund at/above it answers 202 {approval_request} and refunds nothing;
// 5) the requester cannot approve their own request;
// 6) the Finance member approves: the refund runs once as the requester, the
//    audit names the approver, the ledger owes the rider and the chain verifies;
// 7) the workflow is turned off again (current behaviour restored).
// Run: STAFF_EMAIL=… STAFF_PASSWORD=… API_URL=http://localhost:8092/v1 NEXA_DB=nexa_workflow node tools/e2e-approvals.mjs
import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";

if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a Super Admin on the local stack)");
const api = process.env.API_URL ?? "http://localhost:8080/v1";
const db = process.env.NEXA_DB ?? "nexa";
const THRESHOLD = 1000, SMALL = 300, LARGE = 1200;
const APPROVER = { phone: "+255700440044", email: "e2e-approver@example.com", password: randomBytes(9).toString("base64url") };

const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const pass = (m) => console.log(`PASS ${m}`);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fail = (m) => { throw new Error(m); };
const psql = (sql) => execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-d", db, "-qtAc", sql]).toString().trim();
async function call(method, path, token, body, expect, headers = {}) {
  const r = await fetch(api + path, { method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...headers },
    body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 300)}`);
  return text ? JSON.parse(text) : null;
}
const staffLogin = async (email, password) => (await call("POST", "/auth/staff/login", null, { email, password })).access_token;
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token;
};
const saveRefundWorkflow = (token, enabled) => call("PUT", "/admin/approvals/workflows/trips.refund", token, {
  enabled, reason: enabled ? "e2e: refunds need Finance" : "e2e: back to off",
  definition: { conditions: { amount_min: String(THRESHOLD) }, expire_hours: 24,
    steps: [{ name: "Finance check", mode: "ANY", roles: ["Finance"], approvals: 1 }] } });

// A completed cash trip to refund (same flow as e2e-trip-money).
async function completedTrip() {
  const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
  const active = await call("GET", "/trips/active", rt);
  if (active.trip) await call("POST", `/trips/${active.trip.id}/cancel`, rt, { reason: "other" }).catch(() => {});
  await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
  const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];
  const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: { lat: -6.7764, lng: 39.229 }, dropoff: { lat: -6.7724, lng: 39.2181 } });
  const trip = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CASH" }, 201);
  for (let i = 0; i < 30; i++) { const o = await call("GET", "/driver/offer", dt); if (o?.trip_id === trip.id || o?.offer?.trip_id === trip.id) break; await sleep(500); }
  let t = await call("POST", `/driver/offers/${trip.id}/accept`, dt);
  for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT", "COMPLETED"])
    t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
  await call("PUT", "/driver/status", dt, { online: false });
  return t;
}

const admin = await staffLogin(process.env.STAFF_EMAIL, process.env.STAFF_PASSWORD);
await call("PUT", "/admin/staff", admin, { phone: APPROVER.phone, full_name: "E2E Finance Approver", roles: ["Finance"],
  email: APPROVER.email, password: APPROVER.password, reason: "e2e: second approver" });
const approver = await staffLogin(APPROVER.email, APPROVER.password);
pass("second staff member (Finance) created through the API and signed in");
const roleOf = async () => (await call("GET", "/admin/staff", admin)).data.find((s) => s.phone === APPROVER.phone).staff_roles;
await call("PUT", "/admin/staff", admin, { phone: APPROVER.phone, roles: ["Finance", "Support"], reason: "e2e: two roles" });
if ((await roleOf()).join() !== "Finance,Support") fail(`multiple roles: ${await roleOf()}`);
await call("PUT", "/admin/staff", admin, { phone: APPROVER.phone, roles: ["Finance"], reason: "e2e: back to one" });
pass("a staff member holds several roles at once (Finance + Support), then back to Finance");

const wf = await saveRefundWorkflow(admin, true);
if (!wf.workflow.enabled || wf.history.length < 1) fail("workflow not saved");
pass(`refund workflow on (v${wf.workflow.version}): amount ≥ ${THRESHOLD}, one Finance step`);

try {
  const t = await completedTrip();
  log(`completed trip ${t.id.slice(0, 8)}; fare ${t.final_fare ?? t.estimated_fare}`);
  const refund = (amount, key) => fetch(`${api}/admin/trips/${t.id}/refunds`, { method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${admin}`, "Idempotency-Key": key },
    body: JSON.stringify({ amount: String(amount), borne_by: "PLATFORM", reason: "e2e: rider overcharged" }) });

  const small = await refund(SMALL, `e2e-small-${t.id}`);
  const smallBody = await small.json();
  if (small.status !== 201 || Number(smallBody.refunded) !== SMALL) fail(`below threshold: ${small.status} ${JSON.stringify(smallBody).slice(0, 200)}`);
  pass(`refund ${SMALL} below the threshold applied at once (201, refunded ${smallBody.refunded})`);

  const large = await refund(LARGE, `e2e-large-${t.id}`);
  const held = await large.json();
  if (large.status !== 202 || held.error || held.approval_request?.status !== "PENDING") fail(`at threshold: ${large.status} ${JSON.stringify(held).slice(0, 300)}`);
  const req = held.approval_request;
  const still = await call("GET", `/admin/trips/${t.id}/refunds`, admin);
  if (Number(still.refunded) !== SMALL) fail(`a held refund must not apply (refunded ${still.refunded})`);
  pass(`refund ${LARGE} answered 202 {approval_request ${req.id.slice(0, 8)}} "${req.summary.title}"; nothing refunded yet`);

  const summary = await call("GET", "/admin/approvals/requests/summary", approver);
  if (summary.actionable < 1) fail("the approver's badge must count the request");
  const self = await call("POST", `/admin/approvals/requests/${req.id}/approve`, admin, { note: "my own" }, 403);
  if (self.error.code !== "APPROVAL_SELF") fail(`self approval: ${self.error.code}`);
  pass(`requester approving their own request → 403 ${self.error.code}`);

  const done = await call("POST", `/admin/approvals/requests/${req.id}/approve`, approver, { note: "e2e: checked the trip" });
  if (done.status !== "EXECUTED") fail(`after approval: ${done.status} ${done.error}`);
  const after = await call("GET", `/admin/trips/${t.id}/refunds`, admin);
  if (Number(after.refunded) !== SMALL + LARGE || after.refunds.length !== 2) fail(`refunded ${after.refunded}`);
  const executed = after.refunds.find((r) => Number(r.amount) === LARGE);
  pass(`approved by Finance → EXECUTED; trip refunded ${after.refunded} (${after.refunds.length} refunds)`);

  const audit = psql(`select reason from audit_log where action='trip.refund' and target='trip:${t.id}' and data->>'refund'='${executed.id}'`);
  if (!audit.includes("approved by E2E Finance Approver")) fail(`audit reason: ${audit}`);
  const actor = psql(`select count(*) from audit_log a join users u on u.id=a.actor_id where a.action='trip.refund' and a.target='trip:${t.id}' and a.data->>'refund'='${executed.id}' and u.email<>'${APPROVER.email}'`);
  if (actor !== "1") fail("the refund must be recorded as the requester's");
  pass(`audit: refund recorded as the requester with "${audit.slice(audit.indexOf("(approved"))}"`);

  const owed = psql(`select -sum(e.amount) from ledger_entries e join ledger_accounts a on a.id=e.account_id join ledger_transactions x on x.id=e.transaction_id
    where a.kind='RIDER_REFUNDS' and x.reference='${executed.id}'`);
  if (Number(owed) !== LARGE) fail(`ledger owes the rider ${owed}`);
  if (!(await call("POST", "/admin/finance/verify", admin)).ok) fail("ledger chain broken");
  pass(`ledger: rider owed ${owed} for the approved refund; chain verifies`);

  const again = await call("POST", `/admin/approvals/requests/${req.id}/approve`, approver, { note: "again" }, 409);
  pass(`a decided request takes no more decisions → 409 ${again.error.code}`);
} finally {
  await saveRefundWorkflow(admin, false);
  log("refund workflow off again");
}
console.log("APPROVALS E2E PASSED");
