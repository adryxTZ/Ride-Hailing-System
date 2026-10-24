/**
 * Trip queue rules the driver app needs: which trip frames belong to the queued
 * next trip (they never replace the current trip on screen), and the banner text.
 */
import type { NextTrip } from "@/services/tripQueue";
import type { Trip } from "@/services/trips";

/** True for the trip waiting behind the driver's current one. */
export const isQueuedTrip = (t: Pick<Trip, "queued">) => t.queued === true;

/** Statuses during which the "Next trip queued" banner shows (the driver is finishing). */
const FINISHING_STEPS: Trip["status"][] = ["IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT"];

export const showsNextTrip = (current?: Pick<Trip, "status">) =>
  !!current && FINISHING_STEPS.includes(current.status);

/** "Asha · 1.2 km away" — who and how far, for the compact banner. */
export function nextTripLine(n: NextTrip): string {
  const km = `${(n.pickup_distance_m / 1000).toFixed(1)} km away`;
  return n.rider_first_name ? `${n.rider_first_name} · ${km}` : km;
}

/** Where the next pickup is, or a fallback for a pinned point. */
export const nextTripPlace = (n: NextTrip) => n.pickup.address ?? "Pinned location";
