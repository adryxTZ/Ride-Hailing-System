/**
 * usePaymentMethod — the rider's preferred payment method: loaded once from the
 * device, written to the booking draft, and saved when the rider changes it.
 */
import { useCallback, useEffect } from "react";
import { useBookingStore } from "@/services/bookingStore";
import { loadPaymentMethod, type PaymentMethod, savePaymentMethod } from "@/services/paymentPreference";

let loaded = false;

export function usePaymentMethod() {
  const method = useBookingStore((s) => s.paymentMethod);
  const setMethod = useBookingStore((s) => s.setPaymentMethod);
  useEffect(() => {
    if (loaded) return;
    loaded = true;
    void loadPaymentMethod().then(setMethod);
  }, [setMethod]);
  const choose = useCallback(
    (m: PaymentMethod) => {
      setMethod(m);
      void savePaymentMethod(m);
    },
    [setMethod],
  );
  return { method: method === "MOBILE_MONEY" ? "MOBILE_MONEY" : "CASH", choose } as const;
}
