// Shared helpers for the k6 scripts: API calls, OTP sign-in, synthetic positions.
// k6 runs JavaScript in its own runtime (no Node APIs); see https://grafana.com/docs/k6/.
import http from "k6/http";
import { fail } from "k6";

/** API base, e.g. https://uat-api.<domain>/v1. Never production. */
export const API = __ENV.API || "http://localhost:8080/v1";
/** Fixed OTP code for servers that do not echo codes (NEXA_OTP_TEST_CODE). */
const OTP_CODE = __ENV.OTP_CODE || "0000";
/** Centre of the synthetic fleet (Dar es Salaam city centre, inside the seeded active area). */
export const CENTRE = { lat: -6.8163, lng: 39.2803 };
/** Fleet spread around the centre, in degrees (~5 km). */
const SPREAD_DEG = 0.045;

/** json sends one JSON request with optional bearer token and a name tag for metrics. */
export function json(method, path, token, body, name) {
  const params = {
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    tags: { name: name || path },
  };
  return http.request(method, API + path, body ? JSON.stringify(body) : null, params);
}

/** signIn requests and verifies an OTP; uses the echoed dev code when the server returns one. */
export function signIn(phone, app) {
  const sent = json("POST", "/auth/otp/request", null, { phone, app }, "auth otp request");
  if (sent.status !== 202) fail(`otp request ${phone}: ${sent.status} ${sent.body}`);
  const code = sent.json("dev_code") || OTP_CODE;
  const res = json("POST", "/auth/otp/verify", null, { phone, code, app }, "auth otp verify");
  if (res.status !== 200) fail(`otp verify ${phone}: ${res.status} ${res.body}`);
  return res.json("access_token");
}

/** phoneFor builds the i-th synthetic number of a series, e.g. +255711900001. */
export function phoneFor(prefix, i) {
  return `${prefix}${String(i + 1).padStart(5, "0")}`;
}

/** startPosition spreads drivers deterministically around the centre. */
export function startPosition(i) {
  const angle = (i * 137.5 * Math.PI) / 180; // golden angle: an even spread without clustering
  const r = SPREAD_DEG * Math.sqrt((i % 1000) / 1000);
  return { lat: CENTRE.lat + r * Math.sin(angle), lng: CENTRE.lng + r * Math.cos(angle) };
}

/** step moves a position a few metres, like a car in traffic between two pings. */
export function step(p) {
  const metresPerDeg = 111_000;
  const maxStepM = 60;
  const d = (Math.random() * 2 - 1) * (maxStepM / metresPerDeg);
  return { lat: p.lat + d, lng: p.lng + (Math.random() * 2 - 1) * (maxStepM / metresPerDeg) };
}
