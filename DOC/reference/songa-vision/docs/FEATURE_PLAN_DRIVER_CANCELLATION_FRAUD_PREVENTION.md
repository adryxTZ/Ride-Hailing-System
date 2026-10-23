# Implementation Plan: Driver Cancellation Fraud Prevention System

Comprehensive technical design and step-by-step implementation plan to prevent, detect, and mitigate driver cancellation fraud across the Songa platform.

---

## 1. Executive Summary & Problem Statement

Driver cancellation fraud harms platform revenue, compromises user safety, and degrades service reliability. The primary fraud vectors addressed by this plan are:

1. **Off-App Cash Collusion ("Cancel & Ride Off-App")**: Driver meets rider, asks them to cancel (or driver cancels), and completes the trip off-app for cash to bypass platform commissions.
2. **Ghost Waiting / Cancellation Fee Harvesting**: Driver arrives near (or stays outside) pickup, waits 5 minutes without attempting to contact the rider, cancels as "No-Show", and claims a cancellation fee.
3. **Passive Stalling / Cherry-Picking**: Driver accepts a trip but stays stationary waiting for higher surge/fare requests, forcing the rider to cancel first.
4. **Mock Location / GPS Spoofing**: Driver fakes GPS arrival at pickup to trigger automated arrival/no-show fees remotely.

---

## 2. Proposed System Architecture & Modules

```
[Driver Action: Cancel Trip / Location Ping]
                    │
                    ▼
       ┌──────────────────────────┐
       │   Cancellation Gate &    │
       │   Geofence Validation    │ (50m radius + dwell time + contact check)
       └────────────┬─────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
 [Clean Cancellation]    [Flagged / Fee Denied]
        │                       │
        ▼                       ▼
 ┌──────────────┐      ┌─────────────────────────┐
 │ Background   │      │ Admin Audit & Driver    │
 │ Co-Movement  │      │ Penalty Management      │
 │ Audit Task   │      └─────────────────────────┘
 └──────────────┘
```

---

## 3. Database Schema Updates

### A. Update `TripAction` (`trips/models/TripAction.py`)
Add telemetry and fraud audit fields:
- `distance_to_pickup_meters`: `models.FloatField(null=True, blank=True)` — Distance from driver to pickup at time of cancellation.
- `dwell_time_seconds`: `models.IntegerField(default=0)` — Verified duration driver remained within 50m geofence.
- `call_attempt_made`: `models.BooleanField(default=False)` — Whether in-app/phone contact was initiated.
- `cancellation_fee_granted`: `models.BooleanField(default=False)` — Whether cancellation fee was paid out.
- `fraud_flag`: `models.CharField(max_length=50, null=True, blank=True)` — e.g., `OFF_APP_COLLUSION_SUSPECT`, `GHOST_WAITING`, `PASSIVE_STALL`.

### B. New Model `DriverFraudProfile` (`users/models/DriverFraudProfile.py`)
- `driver`: `models.OneToOneField(Driver, on_delete=models.CASCADE)`
- `cancellation_rate_7d`: `models.FloatField(default=0.0)`
- `cancellation_rate_30d`: `models.FloatField(default=0.0)`
- `off_app_incidents_count`: `models.IntegerField(default=0)`
- `is_suspended_for_fraud`: `models.BooleanField(default=False)`
- `suspended_until`: `models.DateTimeField(null=True, blank=True)`
- `updated_at`: `models.DateTimeField(auto_now=True)`

---

## 4. Technical Implementation Steps

### Phase 1: Geofenced Cancellation Gate (`trips/utils/cancellation_fraud.py`)
Implement strict validation before awarding cancellation fees for "Rider No-Show":
1. **Geofence Radius Check**: Verify driver GPS coordinates are $\le 50\text{ meters}$ from pickup.
2. **Dwell Time Verification**: Verify driver location history shows continuous presence inside geofence for $\ge 300\text{ seconds}$ (5 minutes).
3. **Contact Attempt Check**: Verify a call or SMS log exists between driver and rider during the waiting window.
4. **Fee Eligibility Result**: If any of the 3 conditions fail, the cancellation is recorded, but **the cancellation fee is set to 0.0**.

### Phase 2: Post-Cancellation Co-Movement Telemetry (`trips/tasks/fraud_telemetry.py`)
Detect Off-App Cash Collusion:
1. When a trip is canceled near pickup, enqueue a background Celery task `audit_post_cancellation_co_movement`.
2. Monitor driver and rider location streams for 15 minutes post-cancellation.
3. Compute trajectory correlation ($\text{Distance}(\text{Driver}, \text{Rider}) \le 100\text{m}$ along the planned route).
4. If co-movement is detected for $\ge 3\text{ kilometers}$, flag the trip as **`OFF_APP_COLLUSION_SUSPECT`**, automatically charge the commission from the driver's wallet balance, and notify platform admins.

### Phase 3: Stationary Driver Progress Monitor (`trips/tasks/driver_stalling.py`)
Prevent Passive Stalling:
1. Scheduled Celery task checks all active trips in status `ACCEPTED` / `EN_ROUTE_TO_PICKUP`.
2. If driver location has not moved toward pickup over 3 minutes ($\text{speed} < 2\text{ km/h}$ and distance remaining unchanged):
   - Send push warning to driver: *"Please proceed to pickup."*
3. If stationary for 5 minutes total:
   - Re-assign trip to a new driver without rider penalty.
   - Record `PASSIVE_STALL` penalty on driver profile.

### Phase 4: Mock Location Filtering (`utils/geo.py`)
1. Reject location update payloads carrying OS mock provider flags (`is_mock: true`).
2. Reject sudden GPS jumps exceeding $150\text{ km/h}$ between 5-second location pings.

### Phase 5: Admin Fraud Audit Dashboard (`admin_api/views/fraud.py`)
Provide compliance officers with tools to:
- List flagged trips (`OFF_APP_COLLUSION_SUSPECT`, `GHOST_WAITING`, `PASSIVE_STALL`).
- View side-by-side driver & rider location trajectory maps for canceled trips.
- Issue manual suspensions or overturn false-positive fraud flags.

---

## 5. Verification & Testing Plan

### Automated Tests
- Unit tests for geofence validation logic (`trips/tests/test_cancellation_fraud.py`).
- Trajectory co-movement distance calculation unit tests.
- Driver cancellation rate aggregation tests.

### Manual Verification
- Test cancellation inside vs outside 50m geofence.
- Simulate post-cancellation location movement for driver & rider and verify auto-flagging.
