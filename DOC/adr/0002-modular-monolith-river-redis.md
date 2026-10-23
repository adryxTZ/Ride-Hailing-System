# ADR 0002 · Modular monolith, River jobs on Postgres, Redis pub/sub fan-out

Date: 2026-10-14 · Status: Accepted

## Context
Songa's complexity lives in the bridges between runtimes: Kafka topics between Django and the Java gateway, Celery for every transition, Redis↔DB sync loops. Launch scale is hundreds of concurrent drivers and thousands of trips a day.

## Decision
- One Go binary with roles (`api`, `realtime`, `worker`, `scheduler`), launched as a single container and split by compose when needed.
- Bounded-context modules with a lint-enforced dependency order; cross-module side effects only via `trip_events` (transactional outbox) and registered hooks.
- **River** (Postgres-backed) for jobs, retries, unique and periodic jobs. Jobs are inserted in the same transaction as the state change.
- **Redis pub/sub** on one channel for websocket fan-out across instances; clients replay from `trip_events` by sequence number, so fan-out may be best-effort.

## Alternatives
- asynq on Redis: loses transactional enqueue; the ledger side effects must never be lost.
- NATS JetStream / Kafka: a fourth process with its own disk and backup for throughput we will not approach for years.
- Temporal/Cadence for sagas: the right shape, but 400 MB+ of services against a < 1 GB target.

## Consequences
No broker to operate; dispatch/billing jobs cannot be lost or duplicated; horizontal scaling is a compose change; sequence-numbered replay is mandatory in both apps.
