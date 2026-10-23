/** useNearbyVehicles — live cars around a point, refreshed while the screen is open. */
import { useSession } from "@platform/auth";
import { useQuery } from "@tanstack/react-query";
import { fetchNearby } from "@/services/supply";
import type { Point } from "@/services/trips";

const REFRESH_MS = 10_000;
const GRID = 1e3; // round to ~100 m so tiny GPS jitter reuses the cache

export function useNearbyVehicles(at: Point | undefined, service?: string) {
  const { isSignedIn } = useSession();
  const key = at ? [Math.round(at.lat * GRID), Math.round(at.lng * GRID)] : [];
  return useQuery({
    queryKey: ["nearby", service, ...key],
    queryFn: ({ signal }) => fetchNearby(at as Point, service, signal),
    enabled: isSignedIn && !!at,
    refetchInterval: REFRESH_MS,
    staleTime: REFRESH_MS / 2,
  });
}
