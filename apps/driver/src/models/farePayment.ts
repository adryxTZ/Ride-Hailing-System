/**
 * farePayment — where a finished ride's fare stands, as the driver needs it:
 * wait while the rider approves a mobile-money prompt (no cash then), or collect
 * cash (cash rides, a failed prompt, or the rider switched to cash).
 */
import type { Trip } from "@/services/trips";

export type DriverFareState =
  | { kind: "waiting"; secondsLeft: number }
  | { kind: "collect"; note: string }
  | { kind: "none" };

export function driverFareState(trip: Trip, now: number): DriverFareState {
  const pay = trip.payment;
  if (trip.status !== "AWAITING_PAYMENT" || !pay) return { kind: "none" };
  if (pay.status === "PENDING") {
    const expires = pay.prompt ? Date.parse(pay.prompt.expires_at) : now;
    return { kind: "waiting", secondsLeft: Math.max(0, Math.round((expires - now) / 1000)) };
  }
  if (pay.cash_chosen) return { kind: "collect", note: "The rider will pay you in cash." };
  if (pay.status === "FAILED") {
    return {
      kind: "collect",
      note: "Mobile money didn't go through. The rider can try again, or pay you in cash.",
    };
  }
  return pay.method === "MOBILE_MONEY"
    ? { kind: "collect", note: "Mobile money isn't available for this fare. Collect cash." }
    : { kind: "collect", note: "Collect cash, then confirm below." };
}
