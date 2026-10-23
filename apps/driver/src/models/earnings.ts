/**
 * Earnings from the trip list, using each trip's fare split (driver_earnings,
 * commission). Wallet balance and payouts arrive with the ledger.
 */
import type { Trip } from "@/services/trips";

export interface Earnings {
  trips: number;
  /** What the driver keeps. */
  total: number;
  /** Fares collected from riders. */
  fares: number;
  /** Platform fee (commission) on those fares. */
  commission: number;
  currency: string;
}

const fareOf = (t: Trip) => Number(t.final_fare ?? t.estimated_fare ?? 0);

/** What the driver keeps from a trip (the full fare for trips that predate fare splits). */
export const earnedOn = (t: Trip) => Number(t.fees?.driver_earnings ?? fareOf(t));

/** Completed trips since `from`. */
export function earningsSince(trips: Trip[] | undefined, from: Date): Earnings {
  const done = (trips ?? []).filter((t) => t.status === "COMPLETED" && new Date(t.requested_at) >= from);
  return {
    trips: done.length,
    total: done.reduce((sum, t) => sum + earnedOn(t), 0),
    fares: done.reduce((sum, t) => sum + fareOf(t), 0),
    commission: done.reduce((sum, t) => sum + Number(t.fees?.commission ?? 0), 0),
    currency: done[0]?.currency ?? trips?.[0]?.currency ?? "",
  };
}

export function startOfToday(now = new Date()): Date {
  return new Date(now.getFullYear(), now.getMonth(), now.getDate());
}

/** Monday 00:00 of this week. */
export function startOfWeek(now = new Date()): Date {
  const d = startOfToday(now);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return d;
}

export const money = (v: number, currency: string) => `${currency} ${Math.round(v).toLocaleString("en-US")}`;
