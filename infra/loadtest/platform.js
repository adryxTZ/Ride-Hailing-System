// Load test from DOC/01 §D11: 600 online drivers pinging every 10 s through the driver
// location endpoint, plus about one quote per second with a share of them booked
// (and cancelled again so the fleet stays free). Pass: p95 < 150 ms, < 1 % errors.
//
//   k6 run -e API=https://uat-api.<domain>/v1 infra/loadtest/platform.js
//   k6 run -e DRIVERS=20 -e DURATION=30s infra/loadtest/platform.js          (smoke)
//   docker run --rm -i --network host -v "$PWD/infra/loadtest:/lt" grafana/k6:1.3.0 run /lt/platform.js
//
// Drivers must exist and be approved first: infra/loadtest/provision-drivers.sh.
// Never run against production (it creates trips and moves presence).
import { check, sleep } from "k6";
import { CENTRE, json, phoneFor, signIn, startPosition, step } from "./lib.js";

const DRIVERS = Number(__ENV.DRIVERS || 600);
const RIDERS = Number(__ENV.RIDERS || 20);
const DURATION = __ENV.DURATION || "10m";
const PING_INTERVAL_S = Number(__ENV.PING_INTERVAL_S || 10);
const QUOTES_PER_S = Number(__ENV.QUOTES_PER_S || 1);
/** Share of quotes that go on to book (then cancel). */
const BOOK_RATIO = Number(__ENV.BOOK_RATIO || 0.3);
const DRIVER_PREFIX = __ENV.DRIVER_PREFIX || "+2557119";
const RIDER_PREFIX = __ENV.RIDER_PREFIX || "+2557129";
/** A typical city trip: centre to Msasani. */
const DROPOFF = { lat: -6.7724, lng: 39.2181 };

export const options = {
  setupTimeout: "10m",
  scenarios: {
    drivers: {
      executor: "constant-vus",
      exec: "driverLoop",
      vus: DRIVERS,
      duration: DURATION,
    },
    riders: {
      executor: "constant-arrival-rate",
      exec: "riderFlow",
      rate: QUOTES_PER_S,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: RIDERS,
      maxVUs: RIDERS,
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{scenario:drivers}": ["p(95)<150"],
    "http_req_duration{scenario:riders}": ["p(95)<150"],
    "http_req_duration{name:driver location}": ["p(95)<150"],
    "http_req_duration{name:quotes}": ["p(95)<150"],
    "http_req_duration{name:book trip}": ["p(95)<150"],
    checks: ["rate>0.99"],
  },
};

/** setup signs every driver and rider in once and puts the drivers online. */
export function setup() {
  const drivers = [];
  for (let i = 0; i < DRIVERS; i++) {
    const token = signIn(phoneFor(DRIVER_PREFIX, i), "driver");
    const res = json("PUT", "/driver/status", token, { online: true, location: startPosition(i) }, "driver online");
    check(res, { "driver online": (r) => r.status === 200 });
    drivers.push(token);
  }
  const riders = [];
  for (let i = 0; i < RIDERS; i++) riders.push(signIn(phoneFor(RIDER_PREFIX, i), "rider"));
  return { drivers, riders };
}

/** driverLoop: one VU per driver, one location ping every PING_INTERVAL_S. */
export function driverLoop(data) {
  const index = (__VU - 1) % data.drivers.length;
  const token = data.drivers[index];
  // Desynchronise the fleet so pings arrive evenly rather than in one burst.
  if (__ITER === 0) sleep(Math.random() * PING_INTERVAL_S);
  const pos = step(startPosition(index));
  const res = json("POST", "/driver/location", token, { location: pos, heading: 90, speed_mps: 8 }, "driver location");
  check(res, { "ping accepted": (r) => r.status === 200 });
  sleep(PING_INTERVAL_S);
}

/** riderFlow: quote, and for BOOK_RATIO of them book then cancel. */
export function riderFlow(data) {
  const token = data.riders[(__VU - 1) % data.riders.length];
  const quote = json("POST", "/quotes", token, { service: "taxi", pickup: CENTRE, dropoff: DROPOFF }, "quotes");
  if (!check(quote, { "quote ok": (r) => r.status === 200 })) return;
  if (Math.random() >= BOOK_RATIO) return;
  const option = (quote.json("options") || []).find((o) => o.available);
  if (!option) return;
  const trip = json(
    "POST",
    "/trips",
    token,
    { quote_id: quote.json("quote_id"), vehicle_class: option.vehicle_class, payment_method: "CASH" },
    "book trip",
  );
  // 409 = this rider already has a live trip (another iteration); not a failure of the system.
  check(trip, { "booked or busy": (r) => r.status === 201 || r.status === 409 });
  if (trip.status !== 201) return;
  const cancel = json("POST", `/trips/${trip.json("id")}/cancel`, token, { reason: "other", note: "load test" }, "cancel trip");
  check(cancel, { "cancelled": (r) => r.status === 200 });
}

/** teardown takes the fleet offline so UAT riders do not see ghost drivers. */
export function teardown(data) {
  for (const token of data.drivers) json("PUT", "/driver/status", token, { online: false }, "driver offline");
}
