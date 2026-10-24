/** tripQueue service (driver) — the trip queued after the current one, via the shared client. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type NextTrip = components["schemas"]["NextTrip"];

export async function fetchNextTrip(signal?: AbortSignal): Promise<NextTrip | null> {
  const { data, error } = await api.GET("/driver/trips/next", { signal });
  if (error || !data) throw new Error("next trip failed");
  return data.next_trip ?? null;
}
