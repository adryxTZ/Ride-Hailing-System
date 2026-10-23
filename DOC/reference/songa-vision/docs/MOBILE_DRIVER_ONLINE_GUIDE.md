# Songa Mobile — Driver "Go Online" & Location Guide

For the mobile app team. Explains **how a driver becomes matchable**, the **exact order** the app
must follow, the endpoints/payloads, and how to keep a driver reliably visible. Written after a
server-side fix (Sept 2026) that changed one important behaviour — see §5.

Base: API `https://songa.kalen.co.tz` · WebSocket `wss://songa.kalen.co.tz` (nginx proxies `/ws/`).

---

## 1. The model: "online" = TWO things

A driver is dispatchable only when BOTH are true:
1. **Status = online** — the app called `POST /api/users/driver/status/` with `is_online: true`.
   This writes the driver's full state to the matching pool (including `is_verified`, car type,
   cargo types, rating).
2. **Fresh location** — the app is streaming GPS over the location WebSocket, and the last fix is
   recent (the server drops drivers whose last location is older than a few minutes).

Miss either one and the driver will NOT receive trips, even though the app may look "online".

## 2. REQUIRED ORDER (this is the part that was silently breaking)

**Call go-online FIRST, then start streaming location.**

```
1) POST /api/users/driver/status/   { is_online: true, lat, lng }     <-- establishes state
2) open WS  wss://songa.kalen.co.tz/ws/driver/location/?token=<ACCESS_JWT>
3) stream {type:"location_update", ...} every few seconds
```

Why the order matters (post-fix behaviour): the WS gateway now **only merges** a location ping
into an **already-established** state. If the app streams location **before** (or without) calling
go-online, those pings are **ignored** and the driver is **not** pooled. Previously the gateway
would fabricate a partial state from a bare location ping — which put the driver in the pool but
**without `is_verified`**, so the matcher rejected them: the driver looked online but got no trips
("ghost" driver). That fabrication is now removed, so the go-online call is mandatory and must come
first.

On sign-out / going offline: `POST /api/users/driver/status/ { is_online: false }` **and** close
the location WS.

## 3. Endpoints & payloads

### 3a. Go online / offline  (REST, auth required)
`POST /api/users/driver/status/`  — header `Authorization: Bearer <access>`
```json
// online
{ "is_online": true, "lat": -6.7791, "lng": 39.2384 }
// offline
{ "is_online": false }
```
Responses: `200 {"message":"Driver is now online","data":{driver_id,is_online}}` /
`"Driver is now offline"`. Errors to handle:
- `403 "Only verified drivers can go online"` — driver not KYC-approved.
- `403 {"error":"Your account is on hold due to an outstanding balance...","debt":"...","hold_reason":"..."}` — debt hold; block the online toggle and show the settle-debt message.
- `403 {"error":"Your account is suspended pending fraud review.",...}`
- `400 "Latitude (lat) and longitude (lng) are required when going online"`

Check current status: `GET /api/users/driver/status/` → `{"data":{driver_id,is_online}}`.

### 3b. Location stream  (WebSocket — primary)
Connect: `wss://songa.kalen.co.tz/ws/driver/location/?token=<ACCESS_JWT>`
- **Token is the ACCESS JWT** in the `?token=` query param. A **refresh** token is rejected
  (the gateway requires `token_type=access`). Reconnect with a fresh access token after refresh.
Send, every few seconds while online:
```json
{
  "type": "location_update",
  "driver_id": "USER-20250915-B05854",
  "latitude": -6.7791,
  "longitude": 39.2384,
  "timestamp": "2026-09-09T05:30:00Z",
  "trip_id": "<uuid, only when on a trip>"
}
```
- `timestamp`: the server restamps to its own UTC anyway, but send ISO-8601 UTC.
- `trip_id`: include ONLY when on an active trip (adds a breadcrumb to the trip path).

### 3c. Location (REST fallback, optional)
`POST /api/users/driver/location/  { lat, lng }` (auth) — exists as a fallback, but the WS stream
above is the primary/real-time path. Requires the driver to already be online.

## 4. Keeping a driver reliably visible (app responsibilities)

The server drops drivers whose last location is stale. So the app MUST keep fresh fixes flowing
while online:
1. **Heartbeat**: re-send the last known `location_update` on a timer (~every 20-25s) **even if the
   driver hasn't moved**. Do not rely only on OS "location changed" events — a parked driver or an
   OEM power-saving device (Tecno/Infinix/Xiaomi) will otherwise go silent and be dropped.
2. **Keep running in the background** while online (foreground-service + background location), so a
   screen-off / pocketed phone keeps streaming.
3. **Reconnect logic**: if the location WS drops, reconnect (with a fresh access token) and resume
   streaming. On `AppLifecycleState.resumed`, re-check status and reconnect if needed.
4. **Tell the driver** when they're effectively invisible: if the location WS has been down for
   >~30s, show a banner like *"You appear offline to riders — reconnecting…"* and auto re-run the
   go-online + reconnect. (The driver should NOT have to manually toggle offline/online.)

## 5. What changed on the server (Sept 2026) — context for the team

- The gateway **no longer creates a driver state from a bare location ping**. Go-online must run
  first (it writes the full state); pings only update an existing state. → **Order in §2 is now
  mandatory.**
- The gateway **renews the state's 24h TTL** on each ping, so a continuously-online driver's state
  won't expire.
- A server **self-heal** runs every 3 minutes: it re-verifies online drivers and purges any
  leftover unmatchable "ghost" entries. So even transient bad states auto-recover within ~3 min
  with no driver action. (This is a safety net — the app should still follow §2 and §4.)

None of these require an app change to *function*; they make the existing flow reliable. The app
improvements in §4 are what remove the remaining "online but no trips" edge cases.

## 6. How to verify a driver is actually matchable (for QA/support)

A driver gets trips only if, right now: `driver/status` is online, KYC approved, wallet not on
hold, and a location was sent in the last few minutes. Quick rider-side check:
`POST /api/trip/nearby-drivers/ { latitude, longitude }` from near the driver — the driver should
appear in `data.drivers[]` with a small `age_seconds`. If `age_seconds` is large or the driver is
absent, their location stream stopped (see §4).

## 7. Rider side (for completeness)
- `POST /api/trip/nearby-drivers/` — live map of nearby drivers (now returns `last_seen` +
  `age_seconds` per driver).
- `POST /api/trip/Pre-RequestCarrier/` — fares/availability; `POST /api/trip/RequestCarrier/` —
  book; `GET /api/trip/active/` — current trip or pending-rating prompt.
- Trip lifecycle + WS events (trip_accepted, trip_request, payment_finalized, etc.) are documented
  in `API_REFERENCE_AND_SWAHILI_TRANSLATION.md`.

---
Summary for the team: **go-online (REST) BEFORE streaming location (WS); keep a ~25s heartbeat and
background location; reconnect the WS with a fresh access token; show an "appear offline" banner if
the socket drops.** Do that and drivers are reliably matchable.
