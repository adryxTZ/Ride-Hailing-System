/** supply service — available cars around a point (positions and classes only). Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";
import type { Point } from "@/services/trips";

export type NearbyVehicle = components["schemas"]["NearbyVehicle"];

export async function fetchNearby(
  at: Point,
  service?: string,
  signal?: AbortSignal,
): Promise<NearbyVehicle[]> {
  const { data, error } = await api.GET("/drivers/nearby", {
    params: { query: { lat: at.lat, lng: at.lng, service } },
    signal,
  });
  if (error || !data) throw new Error("nearby failed");
  return data.data;
}
