/**
 * useReceipt — a completed trip's receipt (immutable, so cached for good) and
 * opening its PDF: a fresh signed link each tap, opened in the browser or viewer.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { Linking } from "react-native";
import { fetchReceipt, receiptPdfUrl } from "@/services/receipts";

export function useReceipt(tripId?: string, completed = false) {
  return useQuery({
    queryKey: ["receipt", tripId],
    queryFn: ({ signal }) => fetchReceipt(tripId as string, signal),
    enabled: !!tripId && completed,
    staleTime: Number.POSITIVE_INFINITY,
    retry: 2,
  });
}

export function useOpenReceiptPdf(tripId?: string) {
  return useMutation({
    mutationFn: async (lang: "en" | "sw") => Linking.openURL(await receiptPdfUrl(tripId as string, lang)),
  });
}
