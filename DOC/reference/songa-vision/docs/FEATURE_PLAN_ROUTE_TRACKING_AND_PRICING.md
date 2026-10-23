# Feature Plan: Trip Route Tracking & Dual Pricing Models

> **Author:** Generated from codebase audit — 2026-07-27  
> **Status:** Ready for implementation  
> **Scope:** Backend (Django/Celery/Redis) + Driver App (Flutter)

---

## Overview

Two features are being implemented together because they share a common data source: the
**GPS breadcrumb stream** that the driver's device emits every few seconds.

| Feature | What it solves |
|---------|----------------|
| **A — Route Tracking** | Save the actual path driven so trip history can show a map polyline (admin + rider/driver apps). |
| **B — Dual Pricing Models** | Calculate fare from actual distance covered (GPS odometer), while keeping the existing Google-estimated model available for comparison or alternate trip types. |

---

## Current State (What Already Exists)

> [!IMPORTANT]
> A lot of the infrastructure is already built. The work is connecting the missing wiring.

### What Exists ✅
- **`TripLocationHistory`** model (`trips/models/TripLocationHistory.py`) — partitioned PostGIS table, ready for bulk inserts.
- **`TripActivitySummary`** model (`trips/models/TripActivitySummary.py`) — stores compressed polylines per phase.
- **`archive_trip_path(trip_id)`** Celery task (`users/tasks/sync_tasks.py:284`) — reads `trip:{id}:path` Redis stream, bulk-writes to DB on trip end.
- **Admin `/trips/{id}/route/`** endpoint (`admin_api/views/trips.py:128`) — reads Redis first, DB fallback.
- **`process_driver_location` Celery task** (`users/tasks/location_update.py:96`) — called on every driver ping, broadcasts to WebSocket. **This is the injection point.**

### What Is Missing ❌
| Gap | Impact |
|----|--------|
| Nobody calls `redis_client.xadd("trip:{id}:path", ...)` during a live trip | The Redis stream is always empty -> `archive_trip_path` archives nothing -> `TripLocationHistory` is always empty |
| `archive_trip_path` is never called at trip end | Even if the stream had data, it would never be saved to DB |
| No `trip/{id}/route/` endpoint for rider/driver apps | Apps cannot fetch the route polyline |
| No covered-distance price model | Only Google-estimated distance is used at billing |
| Driver app has no "distance covered" state tracking | There is nothing to feed a GPS-odometer price in real time |

---

## Part A — Trip Route Saving & Display

### A.1 — Backend: Write GPS Breadcrumbs to Redis Stream

**File:** `users/tasks/location_update.py`

**Change:** Inside `process_driver_location()`, after updating the Redis state hash, check if
the driver has an active trip. If yes, append the coordinate to `trip:{id}:path` via `xadd`.

```python
# In process_driver_location(), after update_driver_state_location()

# --- NEW: append to trip breadcrumb stream ---
active_trips = Trip.objects.filter(
    driver__driver_id=driver_id,
    status__in=ACTIVE_TRIP_STATUSES  # already defined in this file
).values_list('trip_id', 'status')

for trip_id, trip_status in active_trips:
    stream_key = f"trip:{trip_id}:path"
    redis_client.xadd(stream_key, {
        'lat': str(lat),
        'lng': str(lng),
        'timestamp': str(timezone.now().timestamp()),
        'status': trip_status,
    })
    redis_client.expire(stream_key, 86400)  # 24h safety TTL
```

> [!NOTE]
> `xadd` is O(1) and non-blocking. The stream auto-trims itself if you add `MAXLEN ~= 10000`
> to the xadd call (roughly 28 hours of 10-second pings). This protects Redis memory.

---

### A.2 — Backend: Call `archive_trip_path` at Trip End

**File:** `trips/tasks/trip_operations.py`

**Change:** At the end of `process_end_trip()`, after the payment is written, enqueue the archive task:

```python
# At the bottom of process_end_trip(), inside the success path
from users.tasks.sync_tasks import archive_trip_path
archive_trip_path.delay(str(trip_id))
```

> [!IMPORTANT]
> This must be called **after** the DB transaction commits (it already is, since it goes after
> `trip.save()`). The task reads the stream and writes to `TripLocationHistory` — it does NOT
> need to be inside the atomic block.

---

### A.3 — Backend: Add `route` Endpoint to Rider/Driver API

**File:** `trips/views/` + `trips/urls.py`

**New endpoint:** `GET /api/trip/history/{trip_id}/route/`

```python
class TripRouteView(APIView):
    """
    Returns the GPS breadcrumb path for a completed (or in-flight) trip.
    - In-flight: reads Redis stream trip:{id}:path
    - Completed: reads TripLocationHistory (PostGIS) + TripActivitySummary polylines
    
    Response shape:
    {
        "trip_id": "...",
        "status": "COMPLETED",
        "source": "database",  # or "redis"
        "covered_distance_meters": 4521,
        "points": [
            {"lat": -6.7924, "lng": 39.2083, "timestamp": "...", "status": "ACCEPTED"},
            ...
        ],
        "polylines": {
            "to_pickup": "encoded...",
            "to_destination": "encoded..."
        }
    }
    """
```

**Permission:** Authenticated user who is either the rider or driver of the trip.

**Logic:**
1. Verify requesting user owns the trip (rider or driver).
2. If trip is active -> read Redis stream `trip:{id}:path`.
3. If trip is completed -> read `TripLocationHistory` + `TripActivitySummary`.
4. Compute `covered_distance_meters` by summing geodetic distances between consecutive points.
5. Return unified response.

**Register in urls.py:**
```python
path('history/<uuid:trip_id>/route/', TripRouteView.as_view(), name='trip_route'),
```

---

### A.4 — Driver App: Display Route on Trip History Map

**File:** `songa_driver_app/lib/ui/tracking/`

**New screen:** `trip_history_map_page.dart`

This screen receives a `tripId` and:
1. Calls `GET /api/trip/history/{trip_id}/route/`.
2. Decodes the `points` array into a `List<LatLng>`.
3. Draws a `Polyline` on a `GoogleMap` widget showing the full driven path.
4. Shows pickup marker, dropoff marker, and a colour-coded path:
   - **Blue** = heading to pickup (`status == ACCEPTED/EN_ROUTE_TO_PICKUP`)
   - **Green** = in transit to destination (`status == STARTED/EN_ROUTE_TO_DESTINATION`)
5. Shows summary card: distance covered, duration, fare paid.

**Integration:** Add a "View Route" button to the existing trip history detail card in
`driver_online_page.dart` (or wherever trip history is shown).

---

## Part B — Dual Pricing Models

### Architecture Decision

Two pricing models co-exist and are selected based on **trip type** or an explicit flag on the trip:

| Model | `pricing_model` value | Distance source | When to use |
|---|---|---|---|
| **GPS-Odometer** | `GPS_DISTANCE` | Sum of `TripLocationHistory` breadcrumbs (actual driven path) | Default for all trips going forward; fair to driver on detours |
| **Route-Estimate** | `ROUTE_ESTIMATE` | Google Maps pickup->dropoff at trip completion (current behaviour) | Legacy / special trip types that need predictable pricing |

---

### B.1 — Backend: Add `pricing_model` Field to Trip

**File:** `trips/models/Trip.py`

```python
GPS_DISTANCE   = 'GPS_DISTANCE'
ROUTE_ESTIMATE = 'ROUTE_ESTIMATE'

PRICING_MODEL_CHOICES = [
    (GPS_DISTANCE,   'GPS Distance (actual route covered)'),
    (ROUTE_ESTIMATE, 'Route Estimate (Google Maps straight route)'),
]

pricing_model = models.CharField(
    max_length=20,
    choices=PRICING_MODEL_CHOICES,
    default=GPS_DISTANCE,
    help_text="Determines how trip_distance_meters is sourced at billing.",
)
```

**Migration:** `python manage.py makemigrations trips` -> `python manage.py migrate`

---

### B.2 — Backend: GPS-Odometer Distance Utility

**New file:** `trips/utils/gps_distance.py`

```python
from math import radians, cos, sin, asin, sqrt

def haversine_meters(lat1, lon1, lat2, lon2) -> float:
    """Great-circle distance in meters between two GPS points."""
    R = 6_371_000  # Earth radius in metres
    φ1, φ2 = radians(lat1), radians(lat2)
    Δφ = radians(lat2 - lat1)
    Δλ = radians(lon2 - lon1)
    a = sin(Δφ / 2)**2 + cos(φ1) * cos(φ2) * sin(Δλ / 2)**2
    return R * 2 * asin(sqrt(a))


def compute_covered_distance_meters(trip_id: str) -> int | None:
    """
    Compute the total GPS-odometer distance for a trip.

    Reads from TripLocationHistory (for completed trips) or from the Redis
    stream trip:{id}:path (for in-flight trips) — whichever has data.

    Filters to only STARTED/EN_ROUTE_TO_DESTINATION breadcrumbs so that the
    driver's commute TO pickup is never billed to the rider.

    Returns distance in metres, or None if no telemetry exists.
    """
    from trips.models.TripLocationHistory import TripLocationHistory

    BILLABLE_STATUSES = {'STARTED', 'EN_ROUTE_TO_DESTINATION'}

    # --- 1. Try DB first (completed trips) ---
    points_qs = (
        TripLocationHistory.objects
        .filter(trip__trip_id=trip_id)
        .order_by('timestamp')
        .values_list('location__y', 'location__x')  # lat, lng
    )

    coords = list(points_qs)

    # --- 2. Fallback to Redis stream (live trips) ---
    if not coords:
        from django_redis import get_redis_connection
        redis_client = get_redis_connection('default')
        messages = redis_client.xrange(f'trip:{trip_id}:path', min='-', max='+')
        coords = []
        for _msg_id, data in messages:
            try:
                status = data.get(b'status', b'').decode()
                if status in BILLABLE_STATUSES:
                    coords.append((
                        float(data[b'lat'].decode()),
                        float(data[b'lng'].decode()),
                    ))
            except Exception:
                continue

    if len(coords) < 2:
        return None

    total = 0.0
    for i in range(1, len(coords)):
        total += haversine_meters(*coords[i - 1], *coords[i])

    return int(total)
```

---

### B.3 — Backend: Update `calculate_final_payment` to Respect `pricing_model`

**File:** `pricing/utils/pricing_algo.py`

**Change in `calculate_final_payment(trip)`:**

```python
# Replace the current fixed Google-Maps distance lookup with:

if trip.pricing_model == Trip.GPS_DISTANCE:
    from trips.utils.gps_distance import compute_covered_distance_meters
    distance_meters = compute_covered_distance_meters(str(trip.trip_id))

    if distance_meters is None:
        logger.warning(
            f"GPS_DISTANCE model: no telemetry for trip {trip.trip_id}, "
            f"falling back to ROUTE_ESTIMATE"
        )
        # Graceful fallback — never silently bill zero
        distance_meters = _get_google_distance_meters(trip)
else:
    # ROUTE_ESTIMATE (existing behaviour)
    distance_meters = _get_google_distance_meters(trip)
```

Extract the existing Google Maps call into a private helper `_get_google_distance_meters(trip)`
so both branches can share it cleanly.

> [!WARNING]
> The GPS fallback to Google-estimate must be logged clearly so ops can audit which trips
> were billed via fallback (telemetry may be missing on very short trips or cold-start pings).

---

### B.4 — Backend: Expose `covered_distance_meters` in Trip History API

**File:** `trips/Serializer/`

Add `covered_distance_meters` and `pricing_model` to `SingleTripDetailHistorySerializer`:

```python
covered_distance_meters = serializers.SerializerMethodField()
pricing_model = serializers.CharField(source='pricing_model')

def get_covered_distance_meters(self, obj):
    summary = getattr(obj, 'activity_summary', None)
    if summary:
        return summary.covered_distance_meters  # see B.5
    return obj.trip_distance_meters  # fallback
```

---

### B.5 — Backend: Store `covered_distance_meters` in `TripActivitySummary`

**File:** `trips/models/TripActivitySummary.py`

Add one field:
```python
covered_distance_meters = models.PositiveIntegerField(
    null=True, blank=True,
    help_text="GPS-odometer distance (STARTED phase only), in metres. "
              "Null if telemetry was unavailable."
)
```

**Populate in `archive_trip_path`:** After computing `history_records`, compute the haversine
sum over the STARTED-phase points and write it to the `TripActivitySummary`.

---

### B.6 — Driver App: "Covered Distance" Toggle with Real-Time Odometer

**File:** `songa_driver_app/lib/ui/tracking/trip_to_destination.dart`

#### State additions
```dart
// New fields in _TripToDestinationState
double _coveredDistanceMeters = 0.0;    // GPS odometer
bool _showCoveredDistance = true;        // toggle state
LatLng? _lastBilledPosition;            // last position used in odometer
```

#### Odometer accumulation
In `_startLocationUpdateTimer()` / `_registerLocationListenerOnce()`, after updating
`_lastDriverPosition`, accumulate distance **only when trip status is billable**:

```dart
// Only count distance after driver starts heading to destination
if (_isBillablePhase && _lastBilledPosition != null) {
  final delta = Geolocator.distanceBetween(
    _lastBilledPosition!.latitude, _lastBilledPosition!.longitude,
    newLatLng.latitude, newLatLng.longitude,
  );
  if (delta < 500) {  // sanity cap: ignore GPS jumps > 500m
    setState(() => _coveredDistanceMeters += delta);
  }
}
_lastBilledPosition = newLatLng;
```

Where `_isBillablePhase` is `true` when the WebSocket/session status is `STARTED` or
`EN_ROUTE_TO_DESTINATION` (mirrors backend `BILLABLE_STATUSES`).

#### Toggle bottom bar (replace current distance/duration row)

```dart
GestureDetector(
  onTap: () => setState(() => _showCoveredDistance = !_showCoveredDistance),
  child: AnimatedSwitcher(
    duration: Duration(milliseconds: 300),
    child: _showCoveredDistance
        ? _buildCoveredDistanceCard()   // shows: "4.2 km covered"
        : _buildApproxPriceCard(),       // shows: "~TZS 8,500"
  ),
)
```

`_buildApproxPriceCard()` uses the `estimated_trip_fare` that was sent in the
`trip_en_route_to_destination` WebSocket event — no extra API call needed.

---

## Implementation Order (Suggested for Claude)

> [!IMPORTANT]
> Do these in order — each step depends on the one before it.

```
Step 1  [Backend]  A.1  -> Write xadd in process_driver_location
Step 2  [Backend]  A.2  -> Call archive_trip_path at trip end
Step 3  [Backend]  B.1  -> Add pricing_model field + migration
Step 4  [Backend]  B.2  -> Create gps_distance.py utility
Step 5  [Backend]  B.5  -> Add covered_distance_meters to TripActivitySummary + migration
Step 6  [Backend]  B.3  -> Update calculate_final_payment to branch on pricing_model
Step 7  [Backend]  A.3  -> New TripRouteView endpoint + url registration
Step 8  [Backend]  B.4  -> Expose covered_distance_meters in serializer
Step 9  [Flutter]  B.6  -> GPS odometer + toggle in trip_to_destination.dart
Step 10 [Flutter]  A.4  -> New trip_history_map_page.dart
```

---

## Key Files Reference

| File | Role |
|------|------|
| `users/tasks/location_update.py` | **Step 1** — inject `xadd` here |
| `trips/tasks/trip_operations.py` | **Step 2** — call `archive_trip_path.delay()` at end |
| `trips/models/Trip.py` | **Step 3** — add `pricing_model` field |
| `trips/utils/gps_distance.py` | **Step 4** — NEW file |
| `trips/models/TripActivitySummary.py` | **Step 5** — add `covered_distance_meters` |
| `pricing/utils/pricing_algo.py` | **Step 6** — branch on `pricing_model` |
| `trips/views/` + `trips/urls.py` | **Step 7** — new `TripRouteView` |
| `trips/Serializer/` | **Step 8** — add fields to serializer |
| `songa_driver_app/lib/ui/tracking/trip_to_destination.dart` | **Step 9** — odometer + toggle |
| `songa_driver_app/lib/ui/tracking/` | **Step 10** — new map history screen |

---

## Constraints & Gotchas

> [!CAUTION]
> **Do NOT alter `archive_trip_path`'s DB write logic** — it already correctly creates
> `TripLocationHistory` records via bulk_create. Just ensure it is called.

> [!WARNING]
> **GPS accuracy filter:** Raw GPS can jump. In `gps_distance.py`, ignore segment deltas
> greater than `500 m` between consecutive points (anything larger is a GPS glitch, not real movement).
> On the Flutter side use the same 500 m cap.

> [!NOTE]
> **Backward compatibility:** Existing completed trips will have `pricing_model = GPS_DISTANCE`
> (the new default) but no `TripLocationHistory` rows. `compute_covered_distance_meters` will
> return `None` and gracefully fall back to Google-estimate. No data loss; no re-billing.

> [!NOTE]
> **Migrations needed:** Step 3 (Trip.pricing_model) and Step 5 (TripActivitySummary.covered_distance_meters)
> each require a new migration. Run `makemigrations trips` after each model change.

---

## Testing Checklist

- [ ] Drive a test trip from ACCEPTED -> COMPLETED: verify `trip:{id}:path` Redis stream has entries.
- [ ] After trip completes: verify `archive_trip_path` runs and `TripLocationHistory` has rows.
- [ ] Call `GET /api/trip/history/{id}/route/` and confirm points are returned.
- [ ] Verify `Trip.trip_distance_meters` is set from GPS odometer (not Google Maps) when `pricing_model = GPS_DISTANCE`.
- [ ] Verify fallback to Google Maps when no telemetry exists for the trip.
- [ ] Flutter toggle: confirm odometer starts accumulating only after STARTED status, not during en-route-to-pickup.
- [ ] Verify existing completed trips (no telemetry) still show correct historical fare.
