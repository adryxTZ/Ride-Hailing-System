# Songa Release Notes - v1.0 (Core Engine)

**Release Date**: May 2026  
**Status**: Beta / Core Foundation Functional

This release establishes the core microservices architecture and real-time foundation for the Songa logistics platform.

---

## 🌟 What's Functional

### 1. Microservices Architecture
- **Orchestrated Stack**: Full containerization with Docker Compose.
- **Dual-Core Backend**: 
    - Django for relational data and business rules.
    - Netty (Java) for high-concurrency WebSocket management.
- **Event Bus**: Apache Kafka handling all cross-service communication.

### 2. Real-Time Trip Engine
- **Trip Lifecycle**: Complete flow from Request -> Broadcast -> Acceptance -> Execution -> Completion.
- **Proximity Search**: H3-based nearby driver search (Resolution 9) for low-latency matching.
- **WebSocket Gateway**: Robust connection management for mobile clients with JWT authentication.

### 3. Financial Foundation
- **Wallet System**: Implemented Driver wallets with Debt/Balance tracking.
- **Service Charge**: Automatic 10% commission calculation on trip completion.
- **Account Holds**: Automated logic to hold driver accounts when debt exceeds 5,000 TZS.

### 4. Infrastructure
- **Geospatial Support**: PostGIS enabled for advanced location analytics.
- **Media Handling**: DigitalOcean Spaces integration for cloud asset storage.
- **Connection Pooling**: PgBouncer implemented to scale database connections.

---

## 🚧 In Progress (Next Milestones)

While the core engine is functional, the following areas are currently under development:

- **LATRA Integration**: Connecting to the Land Transport Regulatory Authority for compliance.
- **Payment Gateways**: Direct integration with Mobile Money (M-Pesa, etc.) and Card providers.
- **Dashboard APIs**: Full suite of endpoints for the administrative staff dashboard.
- **Automated Testing**: Building out the unit and integration test suite (currently in progress).
- **Automated Invoicing**: Generation and storage of PDF invoices for riders and drivers.

---

## 📖 Operational Summary

### How a Trip Functions
1. **Rider Requests**: Sends a POST to `/api/trip/request/`.
2. **Broadcast**: Django pushes a message to Kafka; Netty Gateway broadcasts to nearby drivers' WebSockets.
3. **Acceptance**: A driver accepts via `/api/trip/accept/`. assignments are locked in the DB.
4. **Tracking**: Driver location pings go through WebSockets -> Kafka -> Redis/DB.
5. **Completion**: Upon finishing, Django calculates the 10% split and updates the driver's wallet.

---

## 🛠 Tech Specs
- **Backend**: Django 5.1+, Python 3.11+
- **Real-time**: Netty 4.1+, Java 17
- **Streaming**: Kafka (latest)
- **Database**: PostgreSQL 15+ (PostGIS), Redis 7
