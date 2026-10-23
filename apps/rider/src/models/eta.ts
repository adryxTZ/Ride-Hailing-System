/**
 * Pickup-time estimate from live supply: the nearest available car of a class,
 * at an average city speed. Replaced by routed ETAs from dispatch later.
 */
import type { NearbyVehicle } from "@/services/supply";

const CITY_SPEED_KMH = 18; // Dar es Salaam daytime average
const ROAD_FACTOR = 1.3; // straight line → road distance
const MIN_ETA_MIN = 1;

export function etaMinutes(distanceM: number): number {
  const km = (distanceM / 1000) * ROAD_FACTOR;
  return Math.max(MIN_ETA_MIN, Math.round((km / CITY_SPEED_KMH) * 60));
}

/** Minutes until the nearest car offering `vehicleClass` could arrive, or undefined if none is nearby. */
export function etaForClass(vehicles: NearbyVehicle[] | undefined, vehicleClass: string): number | undefined {
  const nearest = vehicles?.find((v) => v.vehicle_classes.includes(vehicleClass));
  return nearest ? etaMinutes(nearest.distance_m) : undefined;
}
