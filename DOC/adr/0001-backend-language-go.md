# ADR 0001 · Backend language: Go

Date: 2026-10-14 · Status: Accepted

## Context
Songa runs four runtimes (Python/Django, Java WebSocket gateway, Celery, Kafka) across ~11 containers on one droplet. Kalen wants a fast, low-resource, quick-to-develop stack, and asked about C and about what Uber uses.

## Decision
Go for the whole backend. One static binary with role flags; websockets in-process; C used only inside libraries (Uber's H3 via `h3-go`).

## Why
- Services idle in tens of MB; goroutines carry thousands of sockets per instance.
- Fast compile and deploy, simple concurrency, boring well-maintained libraries, strong codegen from OpenAPI.
- Hiring and AI-assisted development are far easier than for Rust or Elixir.
- This is the Uber pattern: Go/Java for services, C/C++ only in hot libraries (H3, routing).

## Alternatives
- **C**: 3–5× development cost, no memory safety, no web/DB ecosystem; resource gain over Go is marginal.
- **Rust**: lowest footprint, slowest to build, hardest to hire for locally.
- **TypeScript (Bun/Node)**: one language with the apps, but 2–4× memory and weaker for CPU-bound pricing/geo.
- **Elixir/Phoenix**: best realtime story, niche hiring pool.

## Consequences
Team ramp-up on Go; strict module boundaries enforced by lint; parity golden tests needed because pricing and ledger logic are re-implemented.
