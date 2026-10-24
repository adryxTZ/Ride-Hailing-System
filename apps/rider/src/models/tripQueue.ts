/**
 * Trip queue (rider side): the assigned driver is finishing another trip nearby
 * and comes next. The pickup estimate comes from the server and already counts
 * the finishing time, so the wait never looks like "no driver found".
 */
import type { Trip } from "@/services/trips";

export const FINISHING_HEADLINE = "Your driver is finishing a trip nearby";

const SECONDS_PER_MINUTE = 60;

/** True while the trip waits behind the driver's current trip. */
export const isFinishingDriver = (trip?: Pick<Trip, "queued"> | null) => trip?.queued === true;

/** "Pickup in ~7 min" from the server's estimate, or a neutral line without one. */
export function finishingEtaText(trip: Pick<Trip, "queued_pickup_eta_s">): string {
  const s = trip.queued_pickup_eta_s;
  if (s == null) return "Coming to you next";
  return `Pickup in ~${Math.max(1, Math.round(s / SECONDS_PER_MINUTE))} min`;
}
