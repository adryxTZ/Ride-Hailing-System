/** receipts service — a trip's receipt and a short-lived signed link to its PDF. Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

export type Receipt = components["schemas"]["Receipt"];

export async function fetchReceipt(tripId: string, signal?: AbortSignal): Promise<Receipt> {
  const { data, error } = await api.GET("/trips/{id}/receipt", { params: { path: { id: tripId } }, signal });
  if (error || !data) throw new Error("receipt failed");
  return data;
}

/** A signed PDF address valid for 15 minutes, in English or Swahili. */
export async function receiptPdfUrl(tripId: string, lang: "en" | "sw"): Promise<string> {
  const { data, error } = await api.GET("/trips/{id}/receipt/link", {
    params: { path: { id: tripId }, query: { lang } },
  });
  if (error || !data) throw new Error("Could not open the receipt");
  return data.url;
}
