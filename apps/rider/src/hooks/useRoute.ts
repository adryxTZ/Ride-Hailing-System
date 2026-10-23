/**
 * useRoute — the road route between two moving points. Coordinates are rounded
 * to ~110 m for the cache key, so a car's GPS jitter does not refetch; a real
 * move of a block or more does. The server caches too (≈11 m cells).
 */
import { useQuery } from "@tanstack/react-query";
import { fetchRoute } from "@/services/routes";

const KEY_PRECISION = 1e3; // 3 decimals ≈ 110 m
const STALE_MS = 30_000;

type LatLng = { lat: number; lng: number };
const k = (p?: LatLng) => (p ? [Math.round(p.lat * KEY_PRECISION), Math.round(p.lng * KEY_PRECISION)] : []);

export function useRoute(from?: LatLng, to?: LatLng, via: LatLng[] = []) {
  return useQuery({
    queryKey: ["route", ...k(from), ...k(to), ...via.flatMap(k)],
    queryFn: ({ signal }) => fetchRoute(from as LatLng, to as LatLng, via, signal),
    enabled: !!from && !!to,
    staleTime: STALE_MS,
    placeholderData: (prev) => prev, // keep the last line while the car moves
  });
}
