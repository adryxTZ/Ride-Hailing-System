/** presence service — go online/offline and stream position. Pure data access over the shared client. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type Point = components["schemas"]["Point"];
export type DriverStatus = components["schemas"]["DriverStatus"];
export type DriverStatusInput = components["schemas"]["DriverStatusInput"];
export type LocationSample = components["schemas"]["LocationSample"];

function message(error: unknown, fallback: string): string {
  return (error as { error?: { message?: string } } | undefined)?.error?.message ?? fallback;
}

export async function fetchStatus(signal?: AbortSignal): Promise<DriverStatus> {
  const { data, error } = await api.GET("/driver/status", { signal });
  if (error || !data) throw new Error(message(error, "Could not read status"));
  return data;
}

export async function setStatus(body: DriverStatusInput): Promise<DriverStatus> {
  const { data, error } = await api.PUT("/driver/status", { body });
  if (error || !data) throw new Error(message(error, "Could not change status"));
  return data;
}

export async function postLocation(body: LocationSample): Promise<DriverStatus> {
  const { data, error } = await api.POST("/driver/location", { body });
  if (error || !data) throw new Error(message(error, "Location update failed"));
  return data;
}
