/**
 * bookingDetails — pure rules for the details step: "Who is riding?" when the
 * pickup is not where the rider stands (book for someone else), and for a
 * package the receiver (always) plus the sender when the rider is not at the pickup.
 */
import { distanceKm } from "@/models/geo";
import type { PlacePick } from "@/services/bookingStore";
import type { Service } from "@/services/catalog";
import type { Point } from "@/services/trips";

/** A pickup this close to the phone's position counts as "the rider is here". */
const HERE_RADIUS_KM = 0.15;
/** Shortest phone number the server accepts (digits only). */
export const MIN_PHONE_DIGITS = 9;

/** True when the catalogue asks who is riding for pickups away from the rider. */
export function asksWhoRides(service?: Service): boolean {
  return (service?.lifecycle as Record<string, unknown> | undefined)?.pickup_contact === "when_not_here";
}

/** True when the pickup is the rider's own position (GPS pick or within reach of it). */
export function pickupIsHere(pickup?: PlacePick, here?: Point): boolean {
  if (!pickup) return true;
  if (pickup.source === "gps") return true;
  return !!here && distanceKm(here, pickup.point) <= HERE_RADIUS_KM;
}

/** True when the service needs the receiver's name and phone (package delivery). */
export function receiverRequired(service?: Service): boolean {
  return (service?.lifecycle as Record<string, unknown> | undefined)?.dropoff_contact === "required";
}

/** Whether the booking passes through the details step ("Who is riding?" / package details). */
export function needsDetails(service: Service | undefined, pickup?: PlacePick, here?: Point): boolean {
  return receiverRequired(service) || (asksWhoRides(service) && !pickupIsHere(pickup, here));
}

/** A passenger is complete with a name and a plausible phone number. */
export function contactValid(c?: { name?: string; phone?: string }): boolean {
  return !!c?.name?.trim() && (c.phone ?? "").replace(/\D/g, "").length >= MIN_PHONE_DIGITS;
}
