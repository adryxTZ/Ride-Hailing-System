/** receipts service — a short-lived signed link to a trip's receipt PDF. Pure data access. */
import { api } from "@/services/api";

/** A signed PDF address valid for 15 minutes. */
export async function receiptPdfUrl(tripId: string, lang: "en" | "sw" = "en"): Promise<string> {
  const { data, error } = await api.GET("/trips/{id}/receipt/link", {
    params: { path: { id: tripId }, query: { lang } },
  });
  if (error || !data) throw new Error("Could not open the receipt");
  return data.url;
}
