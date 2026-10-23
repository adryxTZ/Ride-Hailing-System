# WebSocket Driver Mismatch Resolution Architecture

This document describes the architectural mismatch between the **Java Netty WebSocket Gateway** and the **Django Backend** regarding driver routing keys, and details the high-performance Redis alignment solution.

---

## 🏗️ System Structural Architecture Flowchart

The diagram below shows the static topology of all components and the numerical step-by-step data flow through the microservice ecosystem.

```mermaid
graph TD
    %% Define Styles
    classDef client fill:#f9f,stroke:#333,stroke-width:2px;
    classDef gateway fill:#bbf,stroke:#333,stroke-width:2px;
    classDef cache fill:#bfb,stroke:#333,stroke-width:2px;
    classDef broker fill:#fdb,stroke:#333,stroke-width:2px;
    classDef backend fill:#fbf,stroke:#333,stroke-width:2px;
    classDef db fill:#ddd,stroke:#333,stroke-width:2px;

    %% Components
    Driver["📱 Driver Mobile App"]:::client
    Rider["📱 Rider Mobile App"]:::client
    Netty["⚡ Java Netty WS Gateway"]:::gateway
    Redis[("💾 Redis Cache (DB 1)<br/>• h3:&lt;hex&gt;:drivers<br/>• driver:&lt;id&gt;:state<br/>• driver:user_to_driver:&lt;uid&gt;")]:::cache
    Kafka{{"📣 Apache Kafka Broker<br/>• inbound_ws_events<br/>• outbound_ws_events"}}:::broker
    Django["🐍 Django Backend & Consumers"]:::backend
    Postgres[("🐘 PostgreSQL Source-of-Truth")]:::db

    %% Connections & Flows
    Driver == "1. WebSocket Connect (JWT user_id=3)" ==> Netty
    Rider == "5. POST RequestCarrier" ==> Django
    
    Netty -.->| "2. O(1) Get driver_id" | Redis
    Redis -.->| "3. Returns 'USER-20250915-B05854'" | Netty
    
    Netty == "4. Location Stream & Spatial Update" ==> Redis
    Netty ==>| "4. Publish Location Update" | Kafka
    
    Kafka ==>| "Consume Inbound Tracking" | Django
    Django ==>| "Bulk DB Updates" | Postgres
    
    Django ==>| "6. Publish 'trip_request' for USER-20250915-B05854" | Kafka
    Kafka ==>| "7. Consume Outbound Events" | Netty
    Netty == "8. Deliver Broadcast (driver_USER-20250915-B05854)" ==> Driver
```

---

## ⏱️ Temporal Event Sequence Diagram

The sequence below illustrates the complete lifecycle across the dynamic handshake alignment, inbound real-time location streaming, and outbound Kafka trip dispatch broadcasting.

```mermaid
sequenceDiagram
    autonumber
    actor Driver as Driver App
    participant Netty as Netty WS Gateway (Java)
    participant Redis as Redis Cache (DB 1)
    participant Kafka as Apache Kafka Bus
    participant Django as Django API / Consumers
    participant DB as PostgreSQL DB

    %% Handshake Phase
    rect rgb(240, 248, 255)
        Note over Driver, DB: Phase 1: Connection & Dynamic Handshake Alignment
        Driver->>Netty: WebSocket Connect (JWT contains user_id = 3)
        Netty->>Redis: O(1) GET driver:user_to_driver:3
        alt Mapping Found
            Redis-->>Netty: "USER-20250915-B05854" (driver_id)
            Netty->>Netty: Join Session Group: driver_USER-20250915-B05854
        else Fallback (Backward Compatibility)
            Redis-->>Netty: [Null]
            Netty->>Netty: Join Session Group: driver_3
        end
    end

    %% Inbound Tracking Phase
    rect rgb(245, 255, 250)
        Note over Driver, DB: Phase 2: Inbound Location Updates & Spatial Indexing
        Driver->>Netty: Send "location_update" (lat, lng)
        Netty->>Redis: HSET driver:USER-20250915-B05854:state
        Netty->>Redis: SADD h3:897b4c8c923ffff:drivers "USER-20250915-B05854"
        Netty->>Kafka: Publish to "inbound_ws_events" (includes driver_id)
        Kafka->>Django: Consume tracking payload
        Django->>DB: UPDATE driver location in PostgreSQL
    end

    %% Outbound Broadcast Phase
    rect rgb(255, 240, 245)
        Note over Driver, DB: Phase 3: Outbound Trip Dispatch & Broadcast
        Django->>Kafka: Publish "outbound_ws_events" to target "driver_USER-20250915-B05854"
        Kafka->>Netty: Consume trip request event
        Netty->>Driver: Deliver "trip_request" payload to active session!
    end
```

---

## Implemented Keys & Cache Architecture

### Redis Mapping Key
*   **Format**: `driver:user_to_driver:<user_id>`
*   **Value**: `<driver_id>` (string)
*   **Expiry**: `86400 seconds` (24 hours)

### Gateway Session Groups
*   **Driver Session Group**: `driver_<driver_id>`
*   **Fallback Group**: `driver_<user_id>`

### Location State Cache
*   **State Key**: `driver:<driver_id>:state`
*   **Spatial Set Key**: `h3:<h3Index>:drivers`
