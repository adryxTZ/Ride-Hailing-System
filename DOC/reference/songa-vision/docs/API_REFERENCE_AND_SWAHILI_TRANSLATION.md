# Songa Mobile API — Reference & Kiswahili Translation Catalog

Full list of every API the **Rider** and **Driver** apps call, with all success responses and
**every user-facing error/message string quoted verbatim**, plus WebSocket events, SMS, and a
ready-to-fill **translation catalog** at the end (§10).

- **Base URL:** `https://songa.kalen.co.tz`  ·  **WebSocket:** `wss://songa.kalen.co.tz` (nginx `/ws/`)
- **Auth:** JWT — send `Authorization: Bearer <access_token>` on authenticated endpoints. Login is
  OTP-based for mobile (§1); email/password `/token/` exists mainly for staff.
- Prefixes: `/api/users/`, `/api/trip/`, `/api/payments/`, `/api/vehicle/`, `/api/config/`, `/api/messaging/`.

## How to do the Swahili translation (important)

The backend returns **English** in the `error` / `message` / `detail` fields. Translate on the
**app side (presentation layer)**:

1. Build an EN→SW map keyed on the **exact English string** (the tables in §10 are that map — fill
   the `Kiswahili` column and load it as your i18n resource).
2. For strings with placeholders (shown as `{value}` / `<detail>`), translate only the **fixed
   part** and re-insert the dynamic value (e.g. `"You can only rate a completed trip (this trip is
   {status})."` → `"Unaweza kutoa alama kwa safari iliyokamilika tu (safari hii ni {status})."`).
3. Some errors are **framework defaults** (DRF/Django) or `<detail>` system text — see §9; translate
   the fixed prefix and leave the interpolated part.
4. **Chat quick-replies already ship with Swahili** and the chat/notification APIs accept `lang=sw`
   (§7) — use that instead of translating those client-side.
5. Recommended follow-up (backend): add a stable `error_code` to responses so the app maps on codes
   instead of English strings (strings can change). Noted, not yet implemented.

Legend: `{x}` / `<x>` = dynamic value inserted at runtime (do not translate the value).

---

# 1. Auth & OTP  (`/api/users/`, `/token/`)

| Method | Path | App | Auth | Purpose |
|---|---|---|---|---|
| POST | `/api/users/generate-otp/` | both | no | Send OTP to existing user (email or phone) |
| POST | `/api/users/verify-otp/` | both | no | Verify OTP → returns JWT `access`+`refresh` |
| POST | `/api/users/register/customer/` | rider | no | Register rider, then OTP |
| POST | `/api/users/register/driver/` | driver | no | Register driver + vehicle + KYC, then OTP |
| POST | `/token/` | both* | no | Email/password login (mainly staff) |
| POST | `/token/refresh/` (also `/api/users/token/refresh/`) | both | no | Refresh access token |
| POST | `/api/users/logout/` | both | yes | Blacklist refresh token |

**generate-otp** — success `200 {"message": "OTP sent successfully"}`. Errors: `400 "email_or_phone is required"` · `429 "Please wait 60 seconds before requesting another OTP"` · `404 "User not found"` · `403 "This account is not registered as a Driver. Please use the Rider app or register as a driver."` · `403 "This account is registered as a Driver. Please use the Driver app."` · `500 "Internal server error"`.

**verify-otp** — success `200 {"status": true, "message": "OTP verified successfully", "access", "refresh"}`. Errors: `400 "email_or_phone is required"` · `429 "Too many failed attempts. Please try again in 10 minutes."` · `429 "Too many failed attempts. You are locked out for 10 minutes."` · same two 403 app-mismatch strings · serializer: `"User not found"`, `"Invalid or expired OTP"`, `"This field is required."`.

**register/customer** — success `200 {"success": true, "message": "Registration successful. Please check your message for the OTP.", "data": {email, phone_number}}`. Errors: DRF/model defaults (e.g. `"user with this email already exists."`, `"This field is required."`).

**register/driver** — success `201 {"success": true, "message": "Driver created successfully, OTP is being sent to phone number", "data": {phone_number, email, driver_id}}`. Errors: `400 "Invalid phone number."` · `400 "This list may not be empty."` (cargo) · `400 "Missing required vehicle photo(s): <keys>"` · `400 "File size must be less than 5MB."` · `400 "File must be a JPEG or PNG image."` · `400 "This is the same image as '<field>'. Each document must be a distinct photo."` · `400 "This is the same image as your existing '<field>'. Each document must be a distinct photo."` · `500 "Internal server error"`.

**/token/ (login)** — success `200 {access, refresh, user_id, full_name, ...}`. Errors: `400 "Invalid email or password"` · `400 "Account is inactive"`.

**/token/refresh/** — success `200 {"access", ["refresh"]}`. Error: `401 {"detail": "Token is invalid or expired", "code": "token_not_valid"}`.

**logout** — success `205 {"message": "Logged out"}`. Error: `400 "refresh token is required"`.

---

# 2. Profile & Driver account  (`/api/users/`)

| Method | Path | App | Auth | Purpose |
|---|---|---|---|---|
| GET/PATCH | `/api/users/profile/` | both | yes | Get / update own profile (driver or rider shape) |
| POST/GET | `/api/users/driver/status/` | driver | yes | Go online/offline; get status |
| POST | `/api/users/driver/location/` | driver | yes | Location ping while online |
| GET | `/api/users/driver/ledger/` | driver | yes | Earnings ledger (`?period=daily|weekly|monthly`, `start_date`, `end_date`) |
| GET | `/api/users/driver/stats/` | driver | yes | Earnings/wallet stats |

**profile GET** → `{"message": "Profile retrieved successfully", "data": {...}}` (driver shape has `driver_id, user, vehicle, rating, rating_summary, ...`; rider shape has `full_name, email, phone_number, user_type:"Rider", is_verified, rating, rating_summary, profile_picture`). Errors: `403 "Only admins can view other drivers' profiles"` · `404 "Driver not found"` · `500 "Internal server error: <detail>"`.
**profile PATCH** → `{"message": "Profile updated successfully", "data": {...}}`. Errors: `400 "A user with this email already exists."` · `400 "A user with this phone number already exists."` · the two KYC "same image" strings · `500 "Internal server error: <detail>"`.

**driver/status POST** → `{"message": "Driver is now online", "data": {driver_id, is_online}}` or `"Driver is now offline"`. Errors: `400 "No driver profile associated with this user"` · `400 "Please provide 'is_online' (true/false)"` · `403 "Only verified drivers can go online"` · `403 {"error": "Your account is suspended pending fraud review.", "suspended_until": <iso|null>}` · `400 "Latitude (lat) and longitude (lng) are required when going online"` · `400 "Invalid latitude or longitude values"` · `500 "H3 indexing failed: <detail>"` · `500 "Internal server error"`.
**driver/status GET** → `{"message": "Driver status fetched successfully", "data": {...}}` · `500 "Internal server error"`.

**driver/location POST** → `{"message": "Location update received and queued"}` or `{"message": "Update received (rate-limited)"}`. Errors: `400 "No driver profile associated with this user"` · `403 "Only online drivers can ping location"` · `400 "Latitude (lat) and longitude (lng) are required and cannot be empty"` · `400 "Invalid latitude or longitude values: <detail>"` · `400 "Latitude must be between -90 and 90 degrees"` · `400 "Longitude must be between -180 and 180 degrees"` · `500 "Failed to index location with H3: <detail>"` · `500 "Internal server error: <detail>"`.

**driver/ledger GET** → array of ledger entries. Error: `403 "Only drivers can view their ledger"`.
**driver/stats GET** → `{"success": true, "data": {stats, wallet}}`. Error: `403 "Only drivers can view their stats"`.

---

# 3. Rider trip flow  (`/api/trip/`)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/trip/nearby-drivers/` | no | Nearby carriers for the live map |
| POST | `/api/trip/Pre-RequestCarrier/` | no | Fares + availability per vehicle type (pre-book) |
| POST | `/api/trip/EstimateTripPriceAllVehicleType/` | no | Price + route polyline per vehicle type |
| POST | `/api/trip/RequestCarrier/` | yes | Create a trip request (booking) |
| GET | `/api/trip/RequestCarrier/status/` | yes | Poll request status (`?trip_id=`) |
| GET | `/api/trip/destination-history/` | yes | Recent/frequent destinations |

**nearby-drivers** → `{"success": true, "message": "Drivers found successfully. Number of drivers: {n}", "data": {"drivers": [...]}}`. Errors: `400 "Invalid coordinates. Latitude and longitude must be numeric."` · `400 "Coordinates out of range."` · `500 "Failed to index location: <detail>"`.

**Pre-RequestCarrier** → `{"status":"success", "message": "Trip preparation complete" | "No drivers available nearby right now; showing indicative prices.", "data": {...}}`. Errors: `400 "pickup_lat, pickup_lon, destination_lat and destination_lon are required and must be numeric."` · `400 "Invalid pickup coordinates."` · `400 "Invalid destination coordinates."` · `500 "Failed to calculate route via Google Maps"` · `500 "H3 Indexing error: <detail>"` · `500 "Internal server error"`.

**EstimateTripPriceAllVehicleType** → array of per-vehicle price objects. Errors: `400 "Invalid coordinates"` · `500 "Server configuration error"` · `400 "Route lookup failed: <status>"` · `400 "No vehicle types available"` · `500 "Internal server error"`.

**RequestCarrier POST** → `{"trip_id", "estimated_fare", "status": "MATCHING", ...}`. Errors: `409 "You already have an active trip request."` · `400 "Missing required fields: <list>"` · `400 "Could not resolve addresses for the provided coordinates."` · `400 "Car type ID '<id>' does not exist"` · `400 "Cargo type '<name>' is not compatible with requested vehicle type '<name>'"` · `400 "Pricing is not configured for the selected vehicle type. Please choose another vehicle type."` · `500 "Failed to index trip pickup location with H3: <detail>"` · `500 "Internal server error: <detail>"`.
**RequestCarrier GET (status)** → `{trip_id, status, driver_id, created_at, updated_at}`. Errors: `400 "trip_id parameter is required"` · `404 "Trip not found"` · `500 "Internal server error"`.

**destination-history** → `{"status":"success", "message": "No completed trips found" | "Found {n} unique destinations", "data": {...}}`. Error: `500 {"status":"error","message":"Internal server error: <detail>"}`. (Address fallbacks in data, translate if shown: `"Location (<lat>, <lon>)"`, `"Location at <lat>, <lon>"`, `"Unknown location"`.)

---

# 4. Driver trip flow  (`/api/trip/`)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/trip/RequestedTrips/` | yes | Open trip requests to accept |
| POST | `/api/trip/AcceptTrip/` | yes | Accept a trip |
| POST | `/api/trip/RejectTrip/` | yes | Decline an offered trip |
| POST | `/api/trip/DriverProximityAlert/` | yes | Tell rider "driver nearby" |
| POST | `/api/trip/TripStartLoading/` | yes | Loading started |
| POST | `/api/trip/TripLoadingFinished/` | yes | Loading done → en route |
| POST | `/api/trip/TripUnloadingStarted/` | yes | Unloading started |
| POST | `/api/trip/TripUnloadingFinished/` | yes | Complete trip, finalize payment |

**AcceptTrip** → `{"status":"success","message":"Trip accepted successfully", ...}`. Errors: `400 "Missing required fields: trip_id, driver_id"` · `403 "Only online drivers can accept trips"` · `404 "Driver not found"` · `404 "Trip not found or not available for acceptance"` · `500 "Internal server error: <detail>"`.

**RejectTrip** → `{"status":"success","message":"Trip rejection is being processed", ...}`. Errors: `400 "Missing required field: trip_id"` · `400 "Missing required field: driver_id"` · `404 "Driver not found"` · `403 "Only online drivers can reject trips"` · `404 "Trip not found or not in REQUESTED status"` · `500 "Internal server error: <detail>"`.

**DriverProximityAlert** → `{"status":"success","message":"Proximity alert broadcasted"}`. Errors: `400 "driver_id and trip_id are required"` · `404 "Trip not found or not in progress"` · `500 "Internal server error"`.

**TripStartLoading** → `{"status":"success","message":"Trip loading started successfully", "loading_duration", ...}`. Errors: `400 "trip_id and driver_id are required"` · `403 "Unauthorized"` · `403 "Only online drivers can start loading"` · `404 "Driver not found"` · `400 "Driver has no assigned vehicle or invalid capacity"` · `404 "Trip not found or not in STARTED status"` · `500 "Internal server error"`.

**TripLoadingFinished** → `{"status":"Trip loading finished","message":"Trip finishing loading is being processed", ...}`. Errors: `400 "Missing required fields"` · `403 "Unauthorized"` · `500 "Internal server error"`.

**TripUnloadingStarted** → `{"status":"Trip unloading started","message":"Trip unloading is being processed", ...}`. Errors: `400 "trip_id and driver_id are required"` · `403 "Unauthorized"` · `500 "Internal server error"`. (Task internal: `"Driver not found"`, `"Trip not found or not en route"`, `"Vehicle/Tonnage error"`.)

**TripUnloadingFinished** → `{"status":"Trip unloading finished","message":"Trip completion is being processed", ...}`. Errors: same 400/403/500 as above.

---

# 5. Shared trip endpoints  (`/api/trip/`)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/trip/active/` | yes | Current live trip, or mandatory pending-rating prompt |
| POST/GET | `/api/trip/ratings/` | yes | Submit / read a rating (direction inferred) |
| GET | `/api/trip/rating-aspects/` | yes | Aspect definitions for the rating form |
| GET | `/api/trip/history/` | yes | Paginated trip history |
| GET | `/api/trip/history/<uuid>/` | yes | Single trip detail + telemetry |
| GET | `/api/trip/trip-actions/` | yes | Rejections/cancellations audit |
| POST | `/api/trip/CancelTrip/` | yes | Cancel a trip (rider or driver) |

**active** → `{"active_trip": <obj|null>, "requires_rating": <action|null>, "pending_rating": <same>}`. The rating action carries `counterparty_name` (fallbacks `"Your rider"` / `"Your driver"`), `mandatory:true`, `aspects`. Error: `500 "Internal server error"`.

**ratings POST** → `201 {"message": "Rating submitted.", "data": {...}}`. Errors: `400 "trip_id is required."` · `404 "Trip not found."` · `400 "You can only rate a completed trip (this trip is {status})."` · `400 "This trip has no driver to rate."` · `403 "You are not a participant of this trip."` · `400 "score must be an integer 1-5."` · `400` dynamic aspect-validation message from config.ratings · `500 "Internal server error: <detail>"`.
**ratings GET** → `{"results":[...]}`. Errors: `404 "Trip not found."` · `403 "You are not authorized to view this trip's ratings."` · `500 "Internal server error: <detail>"`.

**history / single / trip-actions** — errors: `403 "You are not authorized to view this trip's details."` · `404 "Driver not found"` · `400 "Invalid action_type. Must be one of: <list>"` · `500 "Internal server error: <detail>"`.

**CancelTrip** → `{"status":"success","message":"Trip cancellation is being processed","task_id"}`. Errors: `400 "trip_id is required"` · `400 "driver_id is required for driver cancellation"` · `404 "Trip not found"` · `403 "Cancellation is strictly allowed only before loading."` · `400 "Trip is already completed or canceled"` · `403 "You are not authorized to cancel this trip"` · `500 "Internal server error"`.

---

# 6. Payments  (`/api/payments/`)

| Method | Path | App | Purpose |
|---|---|---|---|
| GET/POST/… | `/api/payments/payment-methods/` | both | CRUD saved payment methods (DRF defaults only) |
| GET | `/api/payments/cards/preference/` | rider | Card vs cash state |
| POST | `/api/payments/cards/preference/` | rider | Toggle card charging |
| POST | `/api/payments/cards/register/` | rider | Start card registration (Selcom hosted page) |
| GET/POST | `/api/payments/settlements/` | driver | Debt-settlement history / pay down debt (USSD pay-in) |
| GET/POST | `/api/payments/disbursements/` | driver | Payout history / request withdrawal |

**cards/preference** → `{card_payments_activated, has_card, card, feature_enabled, effective_method}`. POST errors: `400 "Card payments are currently disabled"` · `400 "Add a card to your payment methods first"`.
**cards/register** → `201 {order_id, payment_page_url}`. Errors: `400 "Card payments are currently disabled"` · `400 "Card registration unavailable: <detail>"` · `400 "Could not start card registration; try again later"`.

**settlements POST** → `201 <settlement>` (+ `payin_status` when `pay_now`). Errors: `403 "Only drivers can settle debt"` · `400 "USSD pay-in is currently disabled"` · `400 "Driver has no phone number on file"` · `400 "Settlement amount <amount> exceeds debt <debt>"` · `400 "Invalid settlement amount: <amount>"` · `400 "Driver is required"` · `400 "Could not access driver wallet: <detail>"` · `503 "USSD pay-in is temporarily unavailable. Please try again later."`.
**settlements GET** → array. Error: `403 "Only drivers have settlements"`.

**disbursements POST** → `201 <disbursement>`. Errors: `403 "Only drivers can request withdrawals"` · `400 "Invalid amount"` · `503 "Payouts are temporarily unavailable. Please try again later."` · `400 "Driver has no mobile money payment method on file"` · `400 "Unknown mobile money provider '<provider>' — cannot map to an institution code"` · `400 "<INITIATED_BY> payouts are currently disabled"` · `400 "Invalid payout amount: <amount>"` · `400 "Driver has no wallet"` · `400 "Wallet is on hold (<reason>); payouts blocked"` · `400 "Payout <amount> exceeds wallet balance <balance>"`.
**disbursements GET** → array. Error: `403 "Only drivers have disbursements"`.

> Webhook `POST /api/payments/webhooks/<gateway>/` is provider-only (no JSON body, no strings) — not app-facing.

---

# 7. Messaging / Chat  (`/api/messaging/`) — already supports `lang=sw`

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/messaging/messages/` | Chat history (`?trip_id=&page=&page_size=`) |
| POST | `/api/messaging/messages/` | Send message (REST fallback; `template_key`, `lang`) |
| POST | `/api/messaging/messages/read/` | Mark read |
| GET | `/api/messaging/messages/unread-count/` | Unread badge (`?trip_id=` optional) |
| GET | `/api/messaging/quick-replies/` | One-tap canned messages (`?lang=en|sw`) |
| POST/DELETE | `/api/messaging/devices/` | Register / deactivate FCM push token |

**messages POST** → `201/200 {"message": "Sent.", "data": {...}}`. Errors: `400 "trip_id is required."` · `404 "Trip not found."` · `403 "You are not a participant of this trip."` · `400 "Chat is not open for this trip (status <status>)."` · `400 "client_message_id must be a valid UUID."` · `400 "Message content is empty."` · `400 "Message too long (max 2000 characters)."` · `400 "Unknown quick reply template '<key>'."` · `400 "Quick reply '<key>' is not available for your role."` · `429 "Too many messages, please slow down."` · `500 "Internal server error: <detail>"`.
**messages/read** → `{"marked_read": n}`. Extra error: `404 "up_to_message_id not found on this trip."`.
**quick-replies** → `{"results":[{key,text,sort_order}]}` (`text` already localized via `lang`).
**devices POST** → `{"message": "Device registered."}`. Errors: `400 "A valid token is required."` · `400 "platform must be ANDROID or IOS."` · `400 "app must be RIDER or DRIVER."`.
**devices DELETE** → `{"message": "Device deactivated.", "deactivated": n}`. Error: `400 "token is required."`.

### Seeded quick-replies (already EN + SW — use `?lang=sw`)
| key | role | English | Kiswahili |
|---|---|---|---|
| im_here | DRIVER | I'm here. | Nimefika. |
| arriving_2_min | DRIVER | Arriving in 2 minutes. | Nafika baada ya dakika 2. |
| cant_find_you | DRIVER | I can't find you — please call me. | Sikuoni — tafadhali nipigie simu. |
| on_my_way | RIDER | I'm coming. | Ninakuja. |
| please_wait | RIDER | Please wait a moment. | Tafadhali subiri kidogo. |
| at_pickup | RIDER | I'm at the pickup point. | Nipo eneo la kuchukuliwa. |
| ok | BOTH | OK. | Sawa. |
| thank_you | BOTH | Thank you. | Asante. |

---

# 8. Reference data  (`/api/vehicle/`, `/api/config/`) — all `GET`, no auth, no error strings

`GET /api/vehicle/image-types/` (driver reg photos) · `GET /api/vehicle/cargo-types/` · `GET /api/vehicle/vehicleType/` · `GET /api/vehicle/vehicleMake/` · `GET /api/vehicle/VehicleModel/` · `GET /api/vehicle/VehicleModel/concatenated/` · `GET /api/config/region/` · `GET /api/config/district/` · `GET /api/config/district/concatenated/`.

These return only 200 with **DB-driven** names/descriptions (cargo type names, car type names, region/district names, vehicle-photo labels). If those must appear in Swahili, translate the **database rows** (or add a localized column), not app-side strings.

---

# 9. WebSocket events (`wss://songa.kalen.co.tz`) — messages shown to users

Events carry a `type` and often a human `message`. Translate the `message` values:

| type | message (verbatim) |
|---|---|
| drivers_found | `{n} driver(s) found. Request forwarded to nearby drivers.` |
| no_driver_found | `No available drivers matched this request` |
| no_driver_found | `All available drivers rejected this request` |
| no_driver_found | `No driver accepted the request within {timeout}s` |
| trip_no_longer_available | `This trip request is no longer available` |
| trip_no_longer_available | `This trip request was canceled.` |
| trip_canceled | `Trip has been canceled` (+ ` and is being rerouted.` or `.`) |
| driver_canceled_reassigning | `Your driver canceled. We are finding you a new driver.` |
| driver_proximity_alert | `Your driver is nearby` |
| trip_loading_started_response | `Trip loading started successfully` |
| trip_en_route_to_destination | `Trip now en route to destination` |
| trip_unloading_started | `Trip unloading started` |

Types with **no** message string (state only, nothing to translate): `subscribe_to_trip_channel`,
`trip_request`, `trip_accepted`, `trip_rejected`, `payment_finalized`, `rating_required`,
`trip_channel_close`.

---

# 10. Translation catalog (fill the `Kiswahili` column)

Every unique user-facing English string, deduplicated. `{x}`/`<x>` = runtime value — keep it in the
translation. Framework-default strings (§ bottom) come from Django/DRF, not our code.

### 10.1 Auth / OTP / account
| # | English | Kiswahili |
|---|---|---|
| 1 | OTP sent successfully | |
| 2 | email_or_phone is required | |
| 3 | Please wait 60 seconds before requesting another OTP | |
| 4 | User not found | |
| 5 | This account is not registered as a Driver. Please use the Rider app or register as a driver. | |
| 6 | This account is registered as a Driver. Please use the Driver app. | |
| 7 | OTP verified successfully | |
| 8 | Too many failed attempts. Please try again in 10 minutes. | |
| 9 | Too many failed attempts. You are locked out for 10 minutes. | |
| 10 | Invalid or expired OTP | |
| 11 | Invalid email or password | |
| 12 | Account is inactive | |
| 13 | Registration successful. Please check your message for the OTP. | |
| 14 | Driver created successfully, OTP is being sent to phone number | |
| 15 | Invalid phone number. | |
| 16 | Missing required vehicle photo(s): <keys> | |
| 17 | File size must be less than 5MB. | |
| 18 | File must be a JPEG or PNG image. | |
| 19 | This is the same image as '<field>'. Each document must be a distinct photo. | |
| 20 | This is the same image as your existing '<field>'. Each document must be a distinct photo. | |
| 21 | Logged out | |
| 22 | refresh token is required | |
| 23 | Internal server error | |

### 10.2 Profile & driver account
| # | English | Kiswahili |
|---|---|---|
| 24 | Profile retrieved successfully | |
| 25 | Profile updated successfully | |
| 26 | Only admins can view other drivers' profiles | |
| 27 | Driver not found | |
| 28 | A user with this email already exists. | |
| 29 | A user with this phone number already exists. | |
| 30 | No driver profile associated with this user | |
| 31 | Please provide 'is_online' (true/false) | |
| 32 | Only verified drivers can go online | |
| 33 | Your account is suspended pending fraud review. | |
| 34 | Latitude (lat) and longitude (lng) are required when going online | |
| 35 | Invalid latitude or longitude values | |
| 36 | Driver is now online | |
| 37 | Driver is now offline | |
| 38 | Driver status fetched successfully | |
| 39 | Only online drivers can ping location | |
| 40 | Location update received and queued | |
| 41 | Update received (rate-limited) | |
| 42 | Latitude (lat) and longitude (lng) are required and cannot be empty | |
| 43 | Latitude must be between -90 and 90 degrees | |
| 44 | Longitude must be between -180 and 180 degrees | |
| 45 | Only drivers can view their ledger | |
| 46 | Only drivers can view their stats | |
| 47 | Internal server error: <detail> | |
| 48 | H3 indexing failed: <detail> | |
| 49 | Failed to index location with H3: <detail> | |
| 50 | Invalid latitude or longitude values: <detail> | |

### 10.3 Trips — rider & driver
| # | English | Kiswahili |
|---|---|---|
| 51 | Drivers found successfully. Number of drivers: {n} | |
| 52 | Invalid coordinates. Latitude and longitude must be numeric. | |
| 53 | Coordinates out of range. | |
| 54 | Failed to index location: <detail> | |
| 55 | Trip preparation complete | |
| 56 | No drivers available nearby right now; showing indicative prices. | |
| 57 | pickup_lat, pickup_lon, destination_lat and destination_lon are required and must be numeric. | |
| 58 | Invalid pickup coordinates. | |
| 59 | Invalid destination coordinates. | |
| 60 | Failed to calculate route via Google Maps | |
| 61 | H3 Indexing error: <detail> | |
| 62 | You already have an active trip request. | |
| 63 | Missing required fields: <list> | |
| 64 | Could not resolve addresses for the provided coordinates. | |
| 65 | Car type ID '<id>' does not exist | |
| 66 | Cargo type '<name>' is not compatible with requested vehicle type '<name>' | |
| 67 | Pricing is not configured for the selected vehicle type. Please choose another vehicle type. | |
| 68 | Failed to index trip pickup location with H3: <detail> | |
| 69 | trip_id parameter is required | |
| 70 | Trip not found | |
| 71 | Invalid coordinates | |
| 72 | Server configuration error | |
| 73 | Route lookup failed: <status> | |
| 74 | No vehicle types available | |
| 75 | No completed trips found | |
| 76 | Found {n} unique destinations | |
| 77 | Unknown location | |
| 78 | Trip accepted successfully | |
| 79 | Missing required fields: trip_id, driver_id | |
| 80 | Only online drivers can accept trips | |
| 81 | Trip not found or not available for acceptance | |
| 82 | Trip rejection is being processed | |
| 83 | Missing required field: trip_id | |
| 84 | Missing required field: driver_id | |
| 85 | Only online drivers can reject trips | |
| 86 | Trip not found or not in REQUESTED status | |
| 87 | Proximity alert broadcasted | |
| 88 | driver_id and trip_id are required | |
| 89 | Trip not found or not in progress | |
| 90 | Trip loading started successfully | |
| 91 | Unauthorized | |
| 92 | Only online drivers can start loading | |
| 93 | Driver has no assigned vehicle or invalid capacity | |
| 94 | Trip not found or not in STARTED status | |
| 95 | Trip loading finished | |
| 96 | Trip finishing loading is being processed | |
| 97 | Missing required fields | |
| 98 | Trip now en route to destination | |
| 99 | Trip unloading started | |
| 100 | Trip unloading is being processed | |
| 101 | Trip not found or not en route | |
| 102 | Vehicle/Tonnage error | |
| 103 | Trip unloading finished | |
| 104 | Trip completion is being processed | |
| 105 | Trip cancellation is being processed | |
| 106 | trip_id is required | |
| 107 | driver_id is required for driver cancellation | |
| 108 | Cancellation is strictly allowed only before loading. | |
| 109 | Trip is already completed or canceled | |
| 110 | You are not authorized to cancel this trip | |
| 111 | Invalid action_type. Must be one of: <list> | |

### 10.4 Ratings & active trip
| # | English | Kiswahili |
|---|---|---|
| 112 | Rating submitted. | |
| 113 | trip_id is required. | |
| 114 | Trip not found. | |
| 115 | You can only rate a completed trip (this trip is {status}). | |
| 116 | This trip has no driver to rate. | |
| 117 | You are not a participant of this trip. | |
| 118 | score must be an integer 1-5. | |
| 119 | You are not authorized to view this trip's ratings. | |
| 120 | You are not authorized to view this trip's details. | |
| 121 | Your driver | |
| 122 | Your rider | |

### 10.5 Payments
| # | English | Kiswahili |
|---|---|---|
| 123 | Only drivers have disbursements | |
| 124 | Only drivers can request withdrawals | |
| 125 | Invalid amount | |
| 126 | Payouts are temporarily unavailable. Please try again later. | |
| 127 | Driver has no mobile money payment method on file | |
| 128 | Unknown mobile money provider '<provider>' — cannot map to an institution code | |
| 129 | <INITIATED_BY> payouts are currently disabled | |
| 130 | Invalid payout amount: <amount> | |
| 131 | Driver has no wallet | |
| 132 | Wallet is on hold (<reason>); payouts blocked | |
| 133 | Payout <amount> exceeds wallet balance <balance> | |
| 134 | Only drivers have settlements | |
| 135 | Only drivers can settle debt | |
| 136 | USSD pay-in is currently disabled | |
| 137 | Driver has no phone number on file | |
| 138 | Settlement amount <amount> exceeds debt <debt> | |
| 139 | Invalid settlement amount: <amount> | |
| 140 | Driver is required | |
| 141 | Could not access driver wallet: <detail> | |
| 142 | USSD pay-in is temporarily unavailable. Please try again later. | |
| 143 | Card payments are currently disabled | |
| 144 | Add a card to your payment methods first | |
| 145 | Card registration unavailable: <detail> | |
| 146 | Could not start card registration; try again later | |

### 10.6 Chat / devices
| # | English | Kiswahili |
|---|---|---|
| 147 | Sent. | |
| 148 | Chat is not open for this trip (status <status>). | |
| 149 | client_message_id must be a valid UUID. | |
| 150 | Message content is empty. | |
| 151 | Message too long (max 2000 characters). | |
| 152 | Unknown quick reply template '<key>'. | |
| 153 | Quick reply '<key>' is not available for your role. | |
| 154 | Too many messages, please slow down. | |
| 155 | up_to_message_id not found on this trip. | |
| 156 | Device registered. | |
| 157 | A valid token is required. | |
| 158 | platform must be ANDROID or IOS. | |
| 159 | app must be RIDER or DRIVER. | |
| 160 | Device deactivated. | |
| 161 | token is required. | |

### 10.7 WebSocket messages
| # | English | Kiswahili |
|---|---|---|
| 162 | {n} driver(s) found. Request forwarded to nearby drivers. | |
| 163 | No available drivers matched this request | |
| 164 | All available drivers rejected this request | |
| 165 | No driver accepted the request within {timeout}s | |
| 166 | This trip request is no longer available | |
| 167 | This trip request was canceled. | |
| 168 | Trip has been canceled | |
| 169 | and is being rerouted. | |
| 170 | Your driver canceled. We are finding you a new driver. | |
| 171 | Your driver is nearby | |

### 10.8 SMS (driver lifecycle + OTP) — sent by NextSMS, sender "SONGA"
`{name}` = driver first name, `{id}` = Driver ID, `{reason}` optional.
| # | English (template) | Kiswahili |
|---|---|---|
| 172 | Songa: Hi {name}, your driver account has been APPROVED (Driver ID: {id}). You can now go online and accept trips. Welcome to Songa! | |
| 173 | Songa: Hi {name}, your driver application was not approved. Reason: {reason}. Please correct the issue and resubmit in the app, or contact Songa support. | |
| 174 | Songa: Hi {name}, your driver account (ID: {id}) has been suspended and you cannot go online. Reason: {reason}. Contact Songa support for assistance. | |
| 175 | Songa: Hi {name}, your driver account (ID: {id}) has been reinstated. You can go online again. Welcome back to Songa. | |
| 176 | Songa: Hi {name}, your Songa account access has been disabled and you have been signed out. Reason: {reason}. Contact Songa support if you believe this is an error. | |
| 177 | Songa: Hi {name}, your Songa account access has been restored. You can sign in again. | |
| 178 | Your Songa OTP Code (email subject) | |
| 179 | Your OTP code is: {otp} (email body) | |

> OTP **SMS** body `"<#> Your OTP is {otp} yj04hX41AlP"` — **do NOT translate/alter**; the `<#>` and the hash are Android SMS Retriever tokens the app depends on.

### 10.9 Framework-default strings (Django/DRF — not in our code, but the apps see them)
Translate if you intercept them; they come from validation:
| # | English | Kiswahili |
|---|---|---|
| 180 | This field is required. | |
| 181 | This list may not be empty. | |
| 182 | user with this email already exists. | |
| 183 | Not found. | |
| 184 | Token is invalid or expired | |

---

## Notes / caveats
- `<detail>` / `<status>` interpolations embed raw system text (exceptions, HTTP statuses). Translate
  only the fixed prefix; leave the interpolated value.
- DB-driven names (cargo types, car types, regions, districts, vehicle-photo labels) are **data**,
  not code strings — translate the rows or add a localized column.
- Chat quick-replies and push already support `lang=sw`; prefer that over client-side translation.
- This reflects the API as of 2026-08-28. If the backend later adds `error_code` fields, switch the
  app to map on codes (more stable than English strings).
