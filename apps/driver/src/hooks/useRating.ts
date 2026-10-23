/** useRating — whether the rider has rated a trip, and rating it. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchRatings, type RatingInput, rateTrip } from "@/services/messaging";

export function useRating(tripId: string | undefined, me: "RIDER" | "DRIVER") {
  const qc = useQueryClient();
  const key = ["ratings", tripId ?? ""];
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => fetchRatings(tripId as string, signal),
    enabled: !!tripId,
  });
  const rate = useMutation({
    mutationFn: (body: RatingInput) => rateTrip(tripId as string, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });
  const mine = query.data?.find((r) => r.rater_role === me);
  return { mine, loading: query.isLoading, rate };
}
