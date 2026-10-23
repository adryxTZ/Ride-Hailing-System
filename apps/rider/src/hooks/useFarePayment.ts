/**
 * useFarePayment — what the rider can do about a finished ride's fare: send
 * another mobile-money prompt (to their number or another) or pay cash. The
 * answer is the updated trip, written straight into the trip caches.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { chooseCash, promptFare, type Trip } from "@/services/trips";

export function useFarePayment(tripId?: string) {
  const qc = useQueryClient();
  const write = (trip: Trip) => {
    qc.setQueryData(["trip", trip.id], trip);
    qc.setQueryData(["trip", "active"], trip);
  };
  const prompt = useMutation({
    mutationFn: (phone?: string) => promptFare(tripId as string, phone),
    onSuccess: write,
  });
  const cash = useMutation({ mutationFn: () => chooseCash(tripId as string), onSuccess: write });
  return { prompt, cash };
}
