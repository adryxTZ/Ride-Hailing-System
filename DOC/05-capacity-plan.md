# Capacity plan — 100,000 ride requests a day

Status: measured 2026-10-14 on a development laptop (Intel i9-9880H, 16 threads),
API, Postgres, Redis and the load generator all on the same machine. Production
hardware will do better; treat these numbers as a floor.

## 1. What 100,000 rides a day means per second

| Quantity | Assumption | Peak rate |
|---|---|---|
| Ride requests | 100,000/day, busiest hour = 10 % of the day, ×3 burst inside that hour | ~8 bookings/s |
| Quotes | ~3 per booking (route changes, class browsing) | ~25/s |
| Drivers online at peak | 5,000; ping every 10 s idle, every 4 s on a trip (~3,000 on trips) | ~1,000 pings/s |
| Riders with a map open | 10,000; nearby-cars refresh every 10 s | ~1,000/s |
| Active-trip fallback polls | 10,000 open apps every 10 s (sockets carry the real updates) | ~1,000/s |
| Live sockets | riders + drivers | ~15,000 concurrent |
| Road routes | quotes + trip screens (refetch per ~110 m of movement) | ~150/s, most served from cache |
| Database writes | ~10 rows per ride (trip, stops, events, offers) | ~100/s |

## 2. Measured on one API process

`go run ./tools/load -drivers 3000 -c 64 -d 10s` (backend/tools/load): 3,000
synthetic online drivers, 64 parallel clients per endpoint, 10 s each.

| Endpoint | Requests/s | p50 | p95 | p99 | Needed at peak | Headroom |
|---|---|---|---|---|---|---|
| `GET /services` (cached catalogue) | 23,228 | 2.2 ms | 6.8 ms | 10.0 ms | low | — |
| `GET /drivers/nearby` | 30,285 | 1.6 ms | 4.8 ms | 8.4 ms | ~1,000 | ×30 |
| `POST /driver/location` | 5,274 | 11.9 ms | 18.6 ms | 23.3 ms | ~1,000 | ×5 |
| `GET /trips/active` | 11,025 | 5.7 ms | 7.2 ms | 8.6 ms | ~1,000 | ×11 |
| `POST /quotes` (route cached) | 3,556 | 14.4 ms | 21.7 ms | 38.6 ms | ~25 | ×140 |

API memory after the run: 37 MB. A race-detector build under the same load
reported no data races.

**Conclusion:** one API process covers the 100,000-rides-a-day peak with at
least 5× headroom on every hot path. Production runs two instances behind the
load balancer for redundancy, not for capacity.

## 3. What changed to get here (same day)

| Problem found | Effect | Fix |
|---|---|---|
| Quote schema cache was an unlocked map | Concurrent quotes crashed the whole server (`concurrent map writes`) | Read/write lock (`internal/pricing/service.go`) |
| Nearby search read one Redis set per res-9 cell (217 per search, up to 817 per waiting trip in matching) | 154 req/s, p95 501 ms | Per-service Redis geo index: one `GEOSEARCH` (radius + sort in Redis) + one pipelined read of the nearest records — 1,447 req/s |
| Riders' maps poll nearby constantly | Repeated identical work | 3 s shared result per H3 res-8 cell (~0.7 km²); matching never uses it — 30,285 req/s |
| Stale-driver cleanup existed but was never scheduled | Indexes grow forever | Dispatcher sweeps every 30 s (`seen:{brand}` sorted set) |
| Driver app sent GPS only on movement (25 m) | A parked driver vanished after 2 min while the app said "Online" | Time-based heartbeat (10 s, 4 s on trip); status re-read from the server every 30 s |

## 4. Required before going live at this volume

1. **Self-hosted road routing.** The public OSRM server used in development is
   rate-limited and has no uptime promise. Run OSRM (or Valhalla) with a
   Tanzania OpenStreetMap extract on its own small server (4–8 GB RAM); point
   `NEXA_OSRM_URL` at it. One OSRM core answers thousands of routes per second,
   and the 6-hour route cache (≈11 m cells) absorbs repeats.
2. **Two API instances** (any role mix) behind TLS + a load balancer that
   supports websockets; the hub already fans out through Redis, and only one
   process dispatches per brand (Redis leader lock).
3. **Managed Postgres** with PITR; the hot paths make no Postgres calls per GPS
   ping or per nearby search. Connection pool stays at 20 per instance.
4. **Redis** with persistence off for presence keys is fine; size 1 GB.
5. **Load test in the target environment** with this tool before launch, and
   add sockets (15,000 concurrent) to the tool.

## 5. Watch list (not blocking 100k/day)

- Dispatch matching is a 1 s loop on one leader; it handles thousands of
  waiting trips per tick. Beyond ~1 M rides a day, shard by city/region.
- `trip_events` and `trip_offers` grow by ~1.5 M rows a month at this volume;
  partition monthly before year one.
- Quotes call routing on every new origin/destination pair; if routing slows,
  quotes slow. The fallback to the estimate keeps them working.
