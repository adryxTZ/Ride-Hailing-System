#!/usr/bin/env node
// Response-time baseline for the key endpoints (CLAUDE.md performance standard:
// "p95 latency and queries-per-request are part of the definition of done").
// Signs in a rider (OTP) and a staff member (email + password), calls each endpoint
// N times one after another, and prints p50 / p95 / max per endpoint as a Markdown
// table ready to paste into DOC/perf/.
//
//   STAFF_EMAIL=you@example.com STAFF_PASSWORD=... node tools/measure-p95.mjs
//
// Environment:
//   API             API base (default http://localhost:8080/v1). Local or UAT only.
//   N               calls per endpoint (default 50), after WARMUP calls (default 5)
//   STAFF_EMAIL     dashboard account; STAFF_PASSWORD is required (no default, never committed)
//   RIDER_PHONE     rider used for quotes / trips (default the first OTP test number)
//   OTP_CODE        fixed test code when the server does not echo codes (default 0000)
//   PG_CONTAINER    optional: Postgres container name (e.g. nexa-local-postgres-1). When set,
//                   queries per request are estimated from pg_stat_database transaction
//                   counters, minus the background rate measured while idle (approximate:
//                   each statement outside an explicit transaction counts as one).
// Node 20+ only, no dependencies.
import { execFileSync } from "node:child_process";
import { performance } from "node:perf_hooks";

const API = process.env.API ?? "http://localhost:8080/v1";
const N = Number(process.env.N ?? 50);
const WARMUP = Number(process.env.WARMUP ?? 5);
const RIDER_PHONE = process.env.RIDER_PHONE ?? "+255626512332";
const OTP_CODE = process.env.OTP_CODE ?? "0000";
const PG_CONTAINER = process.env.PG_CONTAINER;
/** How long the background (idle) transaction rate is sampled before measuring. */
const IDLE_WINDOW_MS = 10_000;
const MS_PER_S = 1000;
const P50 = 0.5;
const P95 = 0.95;
/** A quote from the Dar es Salaam city centre to Msasani (inside the seeded area). */
const QUOTE_BODY = { service: "taxi", pickup: { lat: -6.8163, lng: 39.2803 }, dropoff: { lat: -6.7724, lng: 39.2181 } };

/** call performs one request and returns its parsed body; throws on non-2xx. */
async function call(method, path, token, body) {
  const res = await fetch(API + path, {
    method,
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status} ${text.slice(0, 160)}`);
  return text ? JSON.parse(text) : null;
}

/** riderToken signs in with OTP, using the echoed dev code when the server returns one. */
async function riderToken() {
  const sent = await call("POST", "/auth/otp/request", null, { phone: RIDER_PHONE, app: "rider" });
  const code = sent?.dev_code ?? OTP_CODE;
  return (await call("POST", "/auth/otp/verify", null, { phone: RIDER_PHONE, code, app: "rider" })).access_token;
}

/** staffToken signs in to the dashboard; the password only ever comes from the environment. */
async function staffToken() {
  const email = process.env.STAFF_EMAIL;
  const password = process.env.STAFF_PASSWORD;
  if (!email || !password) throw new Error("set STAFF_EMAIL and STAFF_PASSWORD (no defaults)");
  return (await call("POST", "/auth/staff/login", null, { email, password })).access_token;
}

/** percentile returns the q-th percentile (nearest rank) of sorted values. */
function percentile(sorted, q) {
  return sorted[Math.min(sorted.length - 1, Math.ceil(q * sorted.length) - 1)];
}

/** pgTransactions reads the database's committed + rolled back transaction count. */
function pgTransactions() {
  const sql = "select xact_commit + xact_rollback from pg_stat_database where datname = current_database()";
  const out = execFileSync("docker", ["exec", PG_CONTAINER, "psql", "-U", "nexa", "-qtAc", sql]);
  return Number(String(out).trim());
}

/** backgroundRate measures transactions per millisecond while no requests are sent. */
async function backgroundRate(ms) {
  const before = pgTransactions();
  await new Promise((r) => setTimeout(r, ms));
  return (pgTransactions() - before) / ms;
}

/** measure runs one endpoint WARMUP + N times and returns latency stats (and queries/request). */
async function measure(name, fn, idleRate) {
  for (let i = 0; i < WARMUP; i++) await fn();
  const times = [];
  const txBefore = PG_CONTAINER ? pgTransactions() : 0;
  const started = performance.now();
  for (let i = 0; i < N; i++) {
    const t0 = performance.now();
    await fn();
    times.push(performance.now() - t0);
  }
  const elapsed = performance.now() - started;
  const qpr = PG_CONTAINER ? Math.max(0, (pgTransactions() - txBefore - idleRate * elapsed) / N) : undefined;
  times.sort((a, b) => a - b);
  return { name, p50: percentile(times, P50), p95: percentile(times, P95), max: times.at(-1), qpr };
}

/** endpoints lists what is measured; ids come from the admin lists so any database works. */
async function endpoints(rider, staff) {
  const riders = await call("GET", "/admin/riders?limit=1", staff);
  const drivers = await call("GET", "/admin/drivers?limit=1", staff);
  const riderId = riders?.data?.[0]?.id;
  const driverId = drivers?.data?.[0]?.user_id; // drivers are keyed by their user id
  const list = [
    ["GET /app/config", () => call("GET", "/app/config")],
    ["POST /quotes", () => call("POST", "/quotes", rider, QUOTE_BODY)],
    ["GET /trips (rider)", () => call("GET", "/trips", rider)],
    ["GET /admin/overview", () => call("GET", "/admin/overview", staff)],
    ["GET /admin/trips", () => call("GET", "/admin/trips?limit=50", staff)],
  ];
  if (riderId) list.push(["GET /admin/riders/{id}", () => call("GET", `/admin/riders/${riderId}`, staff)]);
  if (driverId) list.push(["GET /admin/drivers/{id}/activity", () => call("GET", `/admin/drivers/${driverId}/activity`, staff)]);
  return list;
}

/** fmt prints milliseconds with one decimal. */
const fmt = (ms) => ms.toFixed(1);

async function main() {
  const [rider, staff] = [await riderToken(), await staffToken()];
  const list = await endpoints(rider, staff);
  const idleRate = PG_CONTAINER ? await backgroundRate(IDLE_WINDOW_MS) : 0;
  const rows = [];
  for (const [name, fn] of list) rows.push(await measure(name, fn, idleRate));

  console.log(`API ${API}, ${N} sequential calls per endpoint after ${WARMUP} warm-up calls`);
  if (PG_CONTAINER) console.log(`background (workers, dispatcher): ${(idleRate * MS_PER_S).toFixed(1)} transactions/s subtracted`);
  console.log("");
  const qHead = PG_CONTAINER ? " queries/req (approx) |" : "";
  console.log(`| Endpoint | p50 ms | p95 ms | max ms |${qHead}`);
  console.log(`|---|---:|---:|---:|${PG_CONTAINER ? "---:|" : ""}`);
  for (const r of rows) {
    const q = r.qpr === undefined ? "" : ` ${r.qpr.toFixed(1)} |`;
    console.log(`| ${r.name} | ${fmt(r.p50)} | ${fmt(r.p95)} | ${fmt(r.max)} |${q}`);
  }
}

main().catch((err) => {
  console.error(`measure-p95: ${err.message}`);
  process.exit(1);
});
