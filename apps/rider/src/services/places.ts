/** places service — place search and reverse names via the API (provider stays server-side). Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";
import type { Point } from "@/services/trips";

export type PlaceResult = components["schemas"]["PlaceResult"];

export async function searchPlaces(q: string, near?: Point, signal?: AbortSignal): Promise<PlaceResult[]> {
  const { data, error } = await api.GET("/places/search", {
    params: { query: { q, lat: near?.lat, lng: near?.lng } },
    signal,
  });
  if (error || !data) throw new Error("place search failed");
  return data.data;
}

export async function reversePlace(at: Point, signal?: AbortSignal): Promise<PlaceResult | null> {
  const { data, error } = await api.GET("/places/reverse", {
    params: { query: { lat: at.lat, lng: at.lng } },
    signal,
  });
  if (error || !data) return null;
  return data.place ?? null;
}
