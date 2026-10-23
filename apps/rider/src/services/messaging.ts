/** messaging service — trip chat, ratings and push-device registration. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type ChatPage = components["schemas"]["ChatPage"];
export type Message = components["schemas"]["Message"];
export type Rating = components["schemas"]["Rating"];
export type RatingInput = components["schemas"]["RatingInput"];
export type DeviceInput = components["schemas"]["DeviceInput"];

const failure = (error: unknown, fallback: string) =>
  new Error((error as { error?: { message?: string } } | undefined)?.error?.message ?? fallback);

export async function fetchChat(tripId: string, signal?: AbortSignal): Promise<ChatPage> {
  const { data, error } = await api.GET("/trips/{id}/messages", { params: { path: { id: tripId } }, signal });
  if (error || !data) throw failure(error, "Could not load the chat");
  return data;
}

export async function sendMessage(tripId: string, body: string, quickReply?: string): Promise<Message> {
  const { data, error } = await api.POST("/trips/{id}/messages", {
    params: { path: { id: tripId } },
    body: { body, quick_reply: quickReply },
  });
  if (error || !data) throw failure(error, "Message not sent");
  return data;
}

export async function fetchRatings(tripId: string, signal?: AbortSignal): Promise<Rating[]> {
  const { data, error } = await api.GET("/trips/{id}/ratings", { params: { path: { id: tripId } }, signal });
  if (error || !data) throw failure(error, "Could not load ratings");
  return data.data;
}

export async function rateTrip(tripId: string, body: RatingInput): Promise<Rating> {
  const { data, error } = await api.POST("/trips/{id}/rating", { params: { path: { id: tripId } }, body });
  if (error || !data) throw failure(error, "Rating not saved");
  return data;
}

export async function registerDevice(body: DeviceInput): Promise<void> {
  const { error } = await api.POST("/me/devices", { body });
  if (error) throw failure(error, "Device not registered");
}
