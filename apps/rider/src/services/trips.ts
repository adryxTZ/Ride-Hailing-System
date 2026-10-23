/** trips service — quotes and bookings via the shared client. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type Quote = components["schemas"]["Quote"];
export type QuoteOption = components["schemas"]["QuoteOption"];
export type Trip = components["schemas"]["Trip"];
export type Point = components["schemas"]["Point"];
export type TripCreate = components["schemas"]["TripCreate"];
export type DriverLocation = components["schemas"]["DriverLocation"];

/** Statuses after which nothing more happens on a trip. */
export const TERMINAL = [
  "COMPLETED",
  "CANCELLED_BY_RIDER",
  "CANCELLED_BY_DRIVER",
  "CANCELLED_BY_STAFF",
  "NO_DRIVER_FOUND",
  "EXPIRED",
];

function message(error: unknown, fallback: string): string {
  return (error as { error?: { message?: string } } | undefined)?.error?.message ?? fallback;
}

/** An API failure that keeps the server's stable error code (screens branch on codes, never on text). */
export class TripError extends Error {
  constructor(
    message: string,
    readonly code?: string,
  ) {
    super(message);
  }
}

function tripError(error: unknown, fallback: string): TripError {
  const e = (error as { error?: { message?: string; code?: string } } | undefined)?.error;
  return new TripError(e?.message ?? fallback, e?.code);
}

export async function requestQuote(body: {
  service: string;
  pickup: Point;
  dropoff?: Point;
  waypoints?: Point[];
  attributes?: Record<string, unknown>;
}): Promise<Quote> {
  const { data, error } = await api.POST("/quotes", { body });
  if (error || !data) throw new Error(message(error, "Could not get a price"));
  return data;
}

export async function createTrip(body: TripCreate): Promise<Trip> {
  const { data, error } = await api.POST("/trips", { body });
  if (error || !data) throw tripError(error, "Could not book");
  return data;
}

export async function fetchActiveTrip(signal?: AbortSignal): Promise<Trip | null> {
  const { data, error } = await api.GET("/trips/active", { signal });
  if (error || !data) throw new Error("active trip failed");
  return data.trip;
}

export async function fetchTrip(id: string, signal?: AbortSignal): Promise<Trip> {
  const { data, error } = await api.GET("/trips/{id}", { params: { path: { id } }, signal });
  if (error || !data) throw new Error(message(error, "Could not load the trip"));
  return data;
}

export async function fetchTrips(signal?: AbortSignal): Promise<Trip[]> {
  const { data, error } = await api.GET("/trips", { signal });
  if (error || !data) throw new Error("trips failed");
  return data.data;
}

export async function cancelTrip(id: string, reason: string, note?: string): Promise<Trip> {
  const { data, error } = await api.POST("/trips/{id}/cancel", {
    params: { path: { id } },
    body: { reason, note },
  });
  if (error || !data) throw new Error(message(error, "Could not cancel"));
  return data;
}

/** What the rider owes from earlier cancellations (added to the next ride), whole currency units. */
export async function fetchDues(signal?: AbortSignal): Promise<number> {
  const { data, error } = await api.GET("/me/dues", { signal });
  if (error || !data) throw new Error("dues failed");
  return Number(data.amount);
}

/** A key for one tap of "try again", so a retried request never sends a second prompt. */
function promptKey(): string {
  return `fare-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

/** Sends another mobile-money prompt for the finished ride's fare (optionally to another number). */
export async function promptFare(id: string, phone?: string): Promise<Trip> {
  const { data, error } = await api.POST("/trips/{id}/payment/prompt", {
    params: { path: { id }, header: { "Idempotency-Key": promptKey() } },
    body: { phone: phone || undefined },
  });
  if (error || !data) throw tripError(error, "Could not send the payment request");
  return data;
}

/** Tells the driver the rider will pay in cash instead. */
export async function chooseCash(id: string): Promise<Trip> {
  const { data, error } = await api.POST("/trips/{id}/payment/cash", { params: { path: { id } } });
  if (error || !data) throw tripError(error, "Could not switch to cash");
  return data;
}
