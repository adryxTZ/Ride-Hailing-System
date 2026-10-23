/** profile service — the signed-in driver's own profile via the shared client. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type Me = components["schemas"]["Me"];

export async function fetchMe(signal?: AbortSignal): Promise<Me> {
  const { data, error } = await api.GET("/me", { signal });
  if (error || !data) throw new Error("me failed");
  return data;
}

export async function updateMe(body: { full_name?: string; locale?: "en" | "sw" }): Promise<Me> {
  const { data, error } = await api.PATCH("/me", { body });
  if (error || !data)
    throw new Error((error as { error?: { message?: string } })?.error?.message ?? "update failed");
  return data;
}
