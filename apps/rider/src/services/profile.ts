/** profile service — own profile and saved places via the shared client. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type Me = components["schemas"]["Me"];
export type Place = components["schemas"]["Place"];
export type PlaceInput = components["schemas"]["PlaceInput"];

export async function fetchMe(signal?: AbortSignal): Promise<Me> {
  const { data, error } = await api.GET("/me", { signal });
  if (error || !data) throw new Error("me failed");
  return data;
}

export async function updateMe(body: {
  full_name?: string;
  email?: string;
  locale?: "en" | "sw";
}): Promise<Me> {
  const { data, error } = await api.PATCH("/me", { body });
  if (error || !data)
    throw new Error((error as { error?: { message?: string } })?.error?.message ?? "update failed");
  return data;
}

export async function fetchPlaces(signal?: AbortSignal): Promise<Place[]> {
  const { data, error } = await api.GET("/me/places", { signal });
  if (error || !data) throw new Error("places failed");
  return data.data;
}

export async function savePlace(body: PlaceInput): Promise<Place> {
  const { data, error } = await api.POST("/me/places", { body });
  if (error || !data) throw new Error("save place failed");
  return data;
}

export async function deletePlace(id: string): Promise<void> {
  const { error } = await api.DELETE("/me/places/{id}", { params: { path: { id } } });
  if (error) throw new Error("delete place failed");
}

export async function fetchCancelReasons(signal?: AbortSignal) {
  const { data, error } = await api.GET("/config/cancel-reasons", { signal });
  if (error || !data) throw new Error("cancel reasons failed");
  return data;
}
