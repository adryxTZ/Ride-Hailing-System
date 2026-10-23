/** safety service — trusted contacts, trip share links and SOS. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type TrustedContact = components["schemas"]["TrustedContact"];
export type ShareLink = components["schemas"]["ShareLink"];
export type SosAlert = components["schemas"]["SosAlert"];
type Point = components["schemas"]["Point"];

const failure = (error: unknown, fallback: string) =>
  new Error((error as { error?: { message?: string } } | undefined)?.error?.message ?? fallback);

export async function fetchTrustedContacts(signal?: AbortSignal): Promise<TrustedContact[]> {
  const { data, error } = await api.GET("/me/trusted-contacts", { signal });
  if (error || !data) throw failure(error, "Could not load your contacts");
  return data.data;
}

export async function saveTrustedContacts(list: TrustedContact[]): Promise<TrustedContact[]> {
  const { data, error } = await api.PUT("/me/trusted-contacts", { body: { data: list } });
  if (error || !data) throw failure(error, "Could not save your contacts");
  return data.data;
}

export async function shareTrip(tripId: string): Promise<ShareLink> {
  const { data, error } = await api.POST("/trips/{id}/share", { params: { path: { id: tripId } } });
  if (error || !data) throw failure(error, "Could not create the link");
  return data;
}

export async function raiseSos(tripId: string, location?: Point): Promise<SosAlert> {
  const { data, error } = await api.POST("/trips/{id}/sos", {
    params: { path: { id: tripId } },
    body: { location },
  });
  if (error || !data) throw failure(error, "Could not send the alert");
  return data;
}
