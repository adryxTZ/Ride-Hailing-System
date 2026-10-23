/**
 * waiting — the pickup waiting clock: free minutes from the driver's arrival,
 * then the per-minute fee quoted on the trip (capped). Pure functions.
 */
import type { Trip } from "@/services/trips";

export interface WaitingTerms {
  graceS: number;
  ratePerMin: number;
  cap: number;
}

/** The waiting terms the trip was booked with (from its quote option). */
export function waitingTerms(trip: Trip): WaitingTerms {
  const o = (trip.quote_option ?? {}) as {
    waiting_grace_s?: number;
    waiting_rate_per_min?: string | number;
    waiting_cap?: string | number;
  };
  return {
    graceS: o.waiting_grace_s ?? 0,
    ratePerMin: Number(o.waiting_rate_per_min ?? 0),
    cap: Number(o.waiting_cap ?? 0),
  };
}

export interface WaitingState {
  /** Seconds since the driver arrived. */
  waitedS: number;
  /** Free seconds left (0 once the fee runs). */
  freeLeftS: number;
  /** Fee so far, whole currency units, capped. */
  fee: number;
  charging: boolean;
}

/** Where the waiting clock stands at `nowMs`, or undefined before the driver arrives. */
export function waitingState(trip: Trip, nowMs: number): WaitingState | undefined {
  const arrived = trip.timestamps?.arrived_pickup_at;
  if (!arrived) return undefined;
  const t = waitingTerms(trip);
  const waitedS = Math.max(0, Math.floor((nowMs - new Date(arrived).getTime()) / 1000));
  const overS = waitedS - t.graceS;
  const minutes = overS > 0 ? Math.ceil(overS / 60) : 0;
  const raw = minutes * t.ratePerMin;
  const fee = Math.round(t.cap > 0 ? Math.min(raw, t.cap) : raw);
  return {
    waitedS,
    freeLeftS: Math.max(0, t.graceS - waitedS),
    fee,
    charging: overS > 0 && t.ratePerMin > 0,
  };
}

/** "2:05". */
export function mmss(s: number): string {
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}
