/** trips service (driver) — offers and the assigned trip via the shared client. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type Offer = components["schemas"]["Offer"];
export type Trip = components["schemas"]["Trip"];
export type TripStatus = components["schemas"]["TripStatus"];

function message(error: unknown, fallback: string): string {
  return (error as { error?: { message?: string } } | undefined)?.error?.message ?? fallback;
}

export async function fetchOffer(signal?: AbortSignal): Promise<Offer | null> {
  const { data, error } = await api.GET("/driver/offer", { signal });
  if (error || !data) throw new Error("offer failed");
  return data.offer ?? null;
}

export async function acceptOffer(tripId: string): Promise<Trip> {
  const { data, error } = await api.POST("/driver/offers/{tripId}/accept", { params: { path: { tripId } } });
  if (error || !data) throw new Error(message(error, "Could not accept"));
  return data;
}

export async function declineOffer(tripId: string): Promise<void> {
  const { error } = await api.POST("/driver/offers/{tripId}/decline", { params: { path: { tripId } } });
  if (error) throw new Error(message(error, "Could not decline"));
}

export async function fetchActiveTrip(signal?: AbortSignal): Promise<Trip | null> {
  const { data, error } = await api.GET("/driver/trips/active", { signal });
  if (error || !data) throw new Error("active trip failed");
  return data.trip ?? null;
}

/** Moves the trip one step; reason "rider_no_show" with CANCELLED_BY_DRIVER ends it as a no-show. */
export async function transitionTrip(id: string, to: TripStatus, reason?: "rider_no_show"): Promise<Trip> {
  const { data, error } = await api.POST("/driver/trips/{id}/transitions", {
    params: { path: { id } },
    body: { to, reason },
  });
  if (error || !data) throw new Error(message(error, "Could not update the trip"));
  return data;
}

export async function fetchTripHistory(signal?: AbortSignal): Promise<Trip[]> {
  const { data, error } = await api.GET("/driver/trips", { signal });
  if (error || !data) throw new Error("trip history failed");
  return data.data;
}
