/**
 * useNextTrip — the trip the driver accepted while finishing the current one.
 * Socket frames about the queued trip and accepts refresh it (useDriverTrips);
 * slow polling is the fallback. Only read while the driver is on a trip.
 */
import { useSession } from "@platform/auth";
import { useQuery } from "@tanstack/react-query";
import { fetchNextTrip } from "@/services/tripQueue";

export const NEXT_TRIP_KEY = ["driver", "next-trip"] as const;
const FALLBACK_POLL_MS = 15_000;

export function useNextTrip(onTrip: boolean) {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: NEXT_TRIP_KEY,
    queryFn: ({ signal }) => fetchNextTrip(signal),
    enabled: isSignedIn && onTrip,
    refetchInterval: FALLBACK_POLL_MS,
  });
}
