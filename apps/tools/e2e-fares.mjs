// Rider fares by mobile money against the real API (fake gateway, NEXA_FAKE_GATEWAY_DELAY=2s):
// 1) a MOBILE_MONEY ride prompts the rider automatically when it ends; the driver
//    cannot take cash while the prompt is open; once collected the trip completes
//    by itself, the driver is credited their earnings and the ledger still verifies;
// 2) a CASH ride: the rider tries mobile money with numbers that fail (insufficient
//    funds, rejected, unanswered), hits the 3-prompt limit, chooses cash, and the
//    driver confirms cash (the platform share becomes the driver's debt);
// 3) card fares are refused at booking.
// Fake outcomes by the last 4 digits of the prompted number: 0001 insufficient,
// 0002 rejected, 0003 never answered, anything else collected.
// Run: node tools/e2e-fares.mjs  (API on :8080 with the fake gateway; Docker Postgres for clock tweaks)
import { execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
if (!process.env.STAFF_EMAIL || !process.env.STAFF_PASSWORD) throw new Error("Needs STAFF_EMAIL and STAFF_PASSWORD (a staff account on the local stack)");

const api = "http://localhost:8080/v1";
const log = (m) => console.log(new Date().toISOString().slice(11, 19), m);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const psql = (sql) =>
  execFileSync("docker", ["exec", "nexa-local-postgres-1", "psql", "-U", "nexa", "-qtAc", sql], { stdio: ["ignore", "pipe", "pipe"] }).toString().trim();
const fail = (m) => { throw new Error(m); };

async function call(method, path, token, body, expect, headers = {}) {
  const r = await fetch(api + path, {
    method,
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...headers },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  if (expect ? r.status !== expect : !r.ok) fail(`${method} ${path} → ${r.status} ${text.slice(0, 300)}`);
  return text ? JSON.parse(text) : null;
}
const sign = async (phone, app) => {
  await call("POST", "/auth/otp/request", null, { phone, app });
  return (await call("POST", "/auth/otp/verify", null, { phone, app, code: "0000" })).access_token;
};
// A number nobody else uses this hour, ending in the fake gateway's outcome digits.
const number = (suffix) => `07${String(Math.floor(Math.random() * 1e4)).padStart(4, "0")}${suffix}`;

const rt = await sign("+255626512332", "rider"), dt = await sign("+255715123702", "driver");
const PICKUP = { lat: -6.7764, lng: 39.229 }, DROPOFF = { lat: -6.7724, lng: 39.2181 };
const active = await call("GET", "/trips/active", rt);
if (active.trip) await call("POST", `/trips/${active.trip.id}/cancel`, rt, { reason: "other" }).catch(() => {});
const driverTrip = await call("GET", "/driver/trips/active", dt);
if (driverTrip.trip) fail(`driver already has trip ${driverTrip.trip.id}; finish it first`);
await call("PUT", "/driver/status", dt, { online: true, location: { lat: -6.777, lng: 39.23 } });
const cls = (await call("GET", "/driver/status", dt)).presence.vehicle_classes[0];

async function rideToEnd(method) {
  const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
  const trip = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: method }, 201);
  for (let i = 0; i < 30; i++) {
    const o = await call("GET", "/driver/offer", dt);
    if (o?.trip_id === trip.id || o?.offer?.trip_id === trip.id) break;
    await sleep(500);
  }
  let t = await call("POST", `/driver/offers/${trip.id}/accept`, dt);
  for (const to of ["EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT"]) {
    t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to });
  }
  return t;
}
const waitFor = async (id, ok, what) => {
  for (let i = 0; i < 40; i++) {
    const t = await call("GET", `/trips/${id}`, rt);
    if (ok(t)) return t;
    await sleep(500);
  }
  fail(`timed out waiting for ${what}`);
};
const balance = async () => Number((await call("GET", "/driver/wallet", dt)).balance);

// 1) mobile money, automatic prompt, collected
const before1 = await balance();
let t = await rideToEnd("MOBILE_MONEY");
// The prompt goes out in the background right after the driver ends the ride.
t = await waitFor(t.id, (x) => x.payment?.status === "PENDING" && x.payment.prompt, "the automatic prompt");
log(`ride ended: fare ${t.payment.amount} ${t.payment.currency}, payment ${t.payment.status}, prompt ${t.payment.prompt?.status}`);
if (t.payment.status !== "PENDING") fail("a mobile-money ride must prompt the rider automatically");
const blocked = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: "COMPLETED" }, 409);
log(`driver tries cash while the prompt is open → ${blocked.error.code}`);
t = await waitFor(t.id, (x) => x.status === "COMPLETED", "the collected fare to complete the trip");
const earned = (await balance()) - before1;
const fees = t.fees;
log(`completed by payment: ${t.payment.status}; driver credited ${earned} (earnings ${fees.driver_earnings})`);
if (t.payment.status !== "COLLECTED") fail("payment status should be COLLECTED");
if (Math.abs(earned - Number(fees.driver_earnings)) > 0.001) fail("driver was not credited their earnings");
const entry = psql(`select t.kind from ledger_transactions t where t.reference='${t.id}' order by created_at`);
if (entry !== "trip.completed") fail(`expected one trip.completed posting, got: ${entry}`);

// Receipt: issued with the completion, JSON for both sides, signed PDF link.
const receipt = await call("GET", `/trips/${t.id}/receipt`, rt);
const sameForDriver = await call("GET", `/trips/${t.id}/receipt`, dt);
log(`receipt ${receipt.number}: ${receipt.lines.map((l) => `${l.label.en} ${l.amount}`).join(", ")} = ${receipt.total} (${receipt.payment.method})`);
if (sameForDriver.number !== receipt.number || Number(receipt.total) !== Number(t.payment.amount)) fail("receipt mismatch");
const link = await call("GET", `/trips/${t.id}/receipt/link?lang=sw`, rt);
const pdf = await fetch(link.url);
const head = Buffer.from(await pdf.arrayBuffer()).subarray(0, 8).toString();
log(`PDF link → ${pdf.status} ${pdf.headers.get("content-type")} ${head}`);
if (pdf.status !== 200 || !head.startsWith("%PDF")) fail("PDF link did not serve a PDF");
const tampered = await fetch(link.url.replace("lang=sw", "lang=en"));
log(`tampered link → ${tampered.status}`);
if (tampered.status !== 403) fail("a tampered link must be refused");
const immutable = (() => {
  try { psql(`update receipts set total = 1 where id='${receipt.id}'`); return false; } catch { return true; }
})();
if (!immutable) fail("an issued receipt was changed");
log("database refuses to change an issued receipt");

// 2) cash ride: failing prompts, limit, cash
const before2 = await balance();
t = await rideToEnd("CASH");
if (t.payment.status !== "DUE" || t.payment.prompt) fail("a cash ride must not prompt anyone");
const key = () => ({ "Idempotency-Key": randomUUID() });
t = await call("POST", `/trips/${t.id}/payment/prompt`, rt, { phone: number("0001") }, 202, key());
t = await waitFor(t.id, (x) => x.payment.status === "FAILED", "insufficient funds");
log(`prompt 1 → ${t.payment.status} (${t.payment.prompt.failure_code}), ${t.payment.prompt.prompts_left} left`);
const k2 = key();
t = await call("POST", `/trips/${t.id}/payment/prompt`, rt, { phone: number("0003") }, 202, k2);
const replay = await call("POST", `/trips/${t.id}/payment/prompt`, rt, { phone: number("0003") }, 202, k2);
if (replay.payment.prompt.prompts_used !== 2) fail("a retried request must not send another prompt");
const second = await call("POST", `/trips/${t.id}/payment/prompt`, rt, { phone: number("0003") }, 409, key());
log(`prompt 2 is open; the same request again → the same prompt; a new one meanwhile → ${second.error.code}`);
const cashEarly = await call("POST", `/trips/${t.id}/payment/cash`, rt, {}, 409);
log(`rider chooses cash while it is open → ${cashEarly.error.code}`);
psql(`update gateway_transactions set expires_at = now() - interval '1 second' where trip_id='${t.id}' and status='PENDING'`);
t = await waitFor(t.id, (x) => x.payment.status === "FAILED", "the unanswered prompt to expire");
log(`prompt 2 → ${t.payment.status} (${t.payment.prompt.failure_code})`);
t = await call("POST", `/trips/${t.id}/payment/prompt`, rt, { phone: number("0002") }, 202, key());
t = await waitFor(t.id, (x) => x.payment.status === "FAILED", "the rejected prompt");
log(`prompt 3 → ${t.payment.prompt.failure_code}, ${t.payment.prompt.prompts_left} left`);
const limit = await call("POST", `/trips/${t.id}/payment/prompt`, rt, {}, 429, key());
log(`prompt 4 → ${limit.error.code}`);
t = await call("POST", `/trips/${t.id}/payment/cash`, rt, {});
log(`rider pays cash: cash_chosen=${t.payment.cash_chosen}`);
t = await call("POST", `/driver/trips/${t.id}/transitions`, dt, { to: "COMPLETED" });
await sleep(800);
const owes = before2 - (await balance());
log(`driver confirmed cash: ${t.status} ${t.payment.status}; driver now owes ${owes} more`);
if (t.payment.status !== "CASH_CONFIRMED") fail("payment status should be CASH_CONFIRMED");
if (Math.abs(owes - (Number(t.fees.total) - Number(t.fees.driver_earnings))) > 0.001) fail("cash trip debt is wrong");

// 3) card refused
const q = await call("POST", "/quotes", rt, { service: "taxi", pickup: PICKUP, dropoff: DROPOFF });
const card = await call("POST", "/trips", rt, { quote_id: q.quote_id, vehicle_class: cls, payment_method: "CARD" }, 400);
log(`card booking → ${card.error.code}`);

// The database refuses a completed trip without a settled fare.
const refused = (() => {
  try { psql(`update trips set payment_status='DUE' where id='${t.id}'`); return false; } catch { return true; }
})();
if (!refused) fail("the database accepted a completed trip with an unpaid fare");
log("database refuses COMPLETED with an unpaid fare");

const staff = (await call("POST", "/auth/staff/login", null, { email: process.env.STAFF_EMAIL, password: process.env.STAFF_PASSWORD })).access_token;
const v = await call("POST", "/admin/finance/verify", staff);
log(`ledger chain: ${JSON.stringify(v)}`);
await call("PUT", "/driver/status", dt, { online: false });
log("FARES E2E PASSED");
