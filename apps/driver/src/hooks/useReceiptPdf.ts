/** useReceiptPdf — opens a completed trip's receipt PDF (a fresh signed link each tap). */
import { useMutation } from "@tanstack/react-query";
import { Linking } from "react-native";
import { receiptPdfUrl } from "@/services/receipts";

export function useReceiptPdf() {
  return useMutation({ mutationFn: async (tripId: string) => Linking.openURL(await receiptPdfUrl(tripId)) });
}
