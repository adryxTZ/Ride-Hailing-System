/** catalog service — the service catalogue from GET /v1/services. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type Service = components["schemas"]["Service"];
export type VehicleClass = components["schemas"]["VehicleClass"];

export async function fetchServices(signal?: AbortSignal): Promise<Service[]> {
  const { data, error } = await api.GET("/services", { signal });
  if (error || !data) throw new Error("services failed");
  return data.data;
}

/** Picks the text for the current locale, falling back to English then any value. */
export function localized(text: Record<string, string> | undefined, locale: string): string {
  if (!text) return "";
  return text[locale] ?? text.en ?? Object.values(text)[0] ?? "";
}
