# ADR 0004 · Dispatch: ETA-based batched assignment with sequential offers and broadcast fallback

Date: 2026-10-14 · Status: Accepted

## Context
Songa broadcasts to the nearest five by straight-line distance, first accept wins, with dead "wave" code. Kalen asked for a more robust engine following Uber's approach. Supply in some regions and hours is thin.

## Decision
- H3 res-9 candidate search with ring expansion and hard filters.
- Cost function on **road ETA** plus acceptance probability, cancel probability, fairness, forward-dispatch penalty and capacity fit; versioned v0 (Songa parity, time-boxed) → v1 (ETA) → v2 (learned).
- **Batched assignment** every 3–5 s solving all open requests in a region together (min-cost bipartite matching).
- **Sequential offers** (12–20 s) to the assigned driver by default; **broadcast** to the top three as an explicit fallback after N rounds or for configured thin-supply regions/hours.
- Forward dispatch for drivers about to finish; manual assignment path for dispatchers; every decision logged in `match_decisions`.

## Alternatives
- Pure broadcast: accept storms, trains drivers to ignore offers, scoring is irrelevant.
- Pure sequential everywhere: starves thin hauling supply.
- Greedy per-request (no batching): fine at one request per minute, wrong as soon as two requests compete for one driver.

## Consequences
Needs an ETA cache (routing provider cost controlled by H3 pair caching); a solver and its tests; richer offer bookkeeping (`trip_assignments`); knobs per service/region/hour.
