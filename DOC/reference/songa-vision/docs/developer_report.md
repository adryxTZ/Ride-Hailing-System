# Songa Technical Developer Report

This document provides a deep-dive into the technical architecture and implementation details of the Songa Backend Microservices project. It is intended for developers who are onboarding or maintaining the system.

---

## 1. System Architecture

The project follows a decoupled, microservices-oriented architecture designed to handle high concurrency and real-time event streaming.

### 🧱 Core Components
- **Django Backend**: Built with Django Rest Framework (DRF) and Daphne (ASGI). It handles the source of truth for the database and complex business rules.
- **Java Netty WS Gateway**: A lightweight, high-performance service dedicated to maintaining WebSocket connections. This offloads the overhead of persistent connections from the Python backend.
- **Apache Kafka**: Acts as the message bus between Django and the Netty Gateway.

---

## 2. Real-time Infrastructure & Kafka

The system uses an **asynchronous event-driven** pattern for real-time features.

### 📤 Outbound Events (Push to Client)
When the Django backend needs to notify a user (e.g., "Trip Accepted"), it uses the `broadcast_to_ws_group` utility:
1. Django publishes a message to the **`outbound_ws_events`** Kafka topic.
2. The message contains a `target_group` (e.g., `driver_123`) and a `payload`.
3. The **Netty WS Gateway** consumes this message, identifies the active WebSocket session for that group, and pushes the payload.

> [!NOTE]
> For details on the real-time driver alignment architecture that maps `user_id` to `driver_id` reactively to route WebSocket channels correctly, see [WebSocket Driver Alignment](file:///Users/imacslim2015/PycharmProjects/SONGA_BACKEND_MICRO_SERVICES/docs/WEBSOCKET_DRIVER_ALIGNMENT.md).

### 📥 Inbound Events (Tracking)
1. Mobile clients send location updates or status pings over WebSockets to the **Netty Gateway**.
2. The Gateway publishes these updates to Kafka topics (e.g., `vehicle_events`).
3. A dedicated **Django Kafka Consumer** (`django_kafka_consumer`) processes these events to update Redis caches and the PostgreSQL database.

---

## 3. Geospatial & H3 Indexing

To optimize "Nearby Driver" searches and real-time tracking, the system uses **Uber's H3 Geospatial Indexing**:
- **Standardized Resolution**: Both the Java Gateway and Python Backend are configured to use **Resolution 9** (approx. 0.1 km² hexagons).
- **Redis Caching**: Active driver locations are cached in Redis indexed by their H3 cell ID. This allows for constant-time lookups of drivers within a specific hexagonal area.
- **PostGIS**: For persistent storage and complex spatial reporting, PostgreSQL is equipped with the PostGIS extension.

---

## 4. Payment & Financial Engine

The payment system is designed to handle both Cash and Digital payments with an integrated wallet system.

- **Service Charge**: The app takes a **10%** commission on every trip.
- **Driver Wallet**:
    - **Debt**: Increases when a driver accepts a cash payment (they owe the 10% commission to the app).
    - **Balance**: Increases when a rider pays via Digital methods (the app owes the 90% share to the driver).
- **Account Hold Logic**:
    - **Threshold**: If a driver's debt exceeds **5,000 TZS**, their account is automatically placed on "Hold".
    - **Restriction**: Drivers on hold cannot accept new trip requests until they settle their debt.
- **Settlements**: Drivers can pay their debt via the app, which is then verified by an admin to clear the hold.

---

## 5. Trip Lifecycle

The trip state machine is managed in the `trips` app.

| State | View / Trigger | Action |
| :--- | :--- | :--- |
| **Requested** | `RequestTripView` | Broadcasted to nearby drivers via Kafka. |
| **Accepted** | `AcceptTripView` | Trip assigned to driver; other broadcasts cancelled. |
| **In Progress** | `TripStartLoading` | Driver has arrived and started the journey. |
| **Finished** | `TripUnloadingFinished` | Payment processed; wallet updated; trip archived. |

---

## 6. Authentication & Security

- **JWT (JSON Web Tokens)**: Used for all API and WebSocket requests.
- **Shared Secret**: Both the Django backend and Java Gateway share the same `SECRET_KEY` to validate tokens independently without constant database lookups.
- **Permissions**: Standard DRF permissions are used for APIs. The Java Gateway validates tokens on the `Upgrade` handshake.

---

## 7. Infrastructure & Deployment

- **PgBouncer**: All database connections go through PgBouncer in `transaction` mode to manage high connection counts efficiently.
- **S3 Storage**: DigitalOcean Spaces is used for storing user profile pictures, vehicle documents, and static assets.
- **NextSMS**: Integrated for OTP (One-Time Password) verification during signup.

---

## 8. Error Monitoring & Ticketing

The application utilizes a dual-channel, high-reliability monitoring setup:
- **Immediate SMTP Admin Alerts**: Standard unhandled 500 exceptions triggered within the ASGI/WSGI web lifecycle are intercepted by Django's `AdminEmailHandler`. Highly detailed trace logs are formatted and emailed via secure SMTP (Gmail) to the defined `ADMINS` tuple.
- **Sentry Issue Ticketing**: Integrated natively using `sentry-sdk` with standard `DjangoIntegration` and `CeleryIntegration`. Any view crash or Celery worker task exception generates a structured, searchable ticket in the Sentry cloud. It includes deep request context, local python variable snapshots, user attributes, and executed SQL query logs.

---

## 9. Background & Automated Scheduled Tasks (Celery)

The system deploys background processing to maintain real-time caches, compile geodetic metrics, and perform self-healing cleanup:

### 🔁 `sync_db_to_redis_task` (Self-Scheduling Loop)
- **Role**: Synchronizes active drivers in PostgreSQL to Redis H3 index pools and sweeps stale connections.
- **Self-Healing**: Iterates through online driver profiles in PostgreSQL, computes current Uber H3 index hashes, writes state tables to Redis (`driver:{id}:state`), and populates geo-query sets (`h3:{index}:drivers`).
- **Garbage Collection**: Checks the last known location update timestamp. If a driver's state hash has not been refreshed in over 10 minutes, the task automatically changes their database profile status to offline, deletes stale Redis keys, and detaches them from their active H3 cell.
- **Scheduling**: Employs distributed Redis locks (`lock:sync_db_to_redis`) to ensure mutual exclusion across scaling worker replicas, and relies on **dynamic self-scheduling execution** by calling `self.apply_async(countdown=180)` at the end of every run.

### 📈 `archive_trip_path` (Post-Trip Telemetry Aggregator)
- **Trigger**: Fired asynchronously in the background as soon as a ride finishes.
- **Mechanism**: Reads all coordinate logs captured during the ride from Redis Streams (`trip:{id}:path`), groups coordinates by status (`ACCEPTED`, `STARTED`), compresses coordinate tracks into polyline segments, computes traffic delays, and writes metrics securely to partitioned Postgres tables (`TripLocationHistory`).

### 📍 `sync_driver_locations_to_db`
- **Schedule**: Scans Redis regularly for active location hashes, compiling bulk changes to persist geodetic spatial updates (`last_known_location`) within PostgreSQL.

### 💬 `send_otp_task`
- **Role**: Asynchronously interacts with the NextSMS API to deliver verification SMS tokens to user profiles during signup.

---

## 🛠 Maintenance & Troubleshooting

### Common Commands
- **Logs**: `docker-compose logs -f [service_name]`
- **Kafka Check**: Use a Kafka UI or CLI to monitor the `outbound_ws_events` topic for throughput.
- **Redis Reset**: `python manage.py verify_redis_caching` (utility script to sync DB with cache).
- **Run Django Check inside Container**: `docker compose exec songa_backend bash -c "source /app/env/bin/activate && python manage.py check"`

### Connectivity Issues
If WebSockets are not receiving messages:
1. Verify the Netty Gateway is successfully connected to the Kafka broker.
2. Check if the `target_group` in the Kafka message matches the group the client joined (usually `driver_{id}` or `rider_{id}`).

