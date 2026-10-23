/**
 * usePlaceSearch — place suggestions while typing: waits for a short pause
 * (DEBOUNCE_MS) and at least MIN_CHARS characters, then asks the API, ranked
 * near the rider. Results are cached by query, so typing back is instant.
 */
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { searchPlaces } from "@/services/places";
import type { Point } from "@/services/trips";

const DEBOUNCE_MS = 350;
const MIN_CHARS = 3;
const NEAR_GRID = 100; // round the bias point to ~1 km so the cache key is stable

export function usePlaceSearch(query: string, near?: Point) {
  const [term, setTerm] = useState(query.trim());
  useEffect(() => {
    const id = setTimeout(() => setTerm(query.trim()), DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [query]);
  const bias = near
    ? { lat: Math.round(near.lat * NEAR_GRID) / NEAR_GRID, lng: Math.round(near.lng * NEAR_GRID) / NEAR_GRID }
    : undefined;
  const q = useQuery({
    queryKey: ["places", term.toLowerCase(), bias?.lat, bias?.lng],
    queryFn: ({ signal }) => searchPlaces(term, bias, signal),
    enabled: term.length >= MIN_CHARS,
    staleTime: 10 * 60_000,
  });
  const typing = query.trim() !== term;
  return {
    results: term.length >= MIN_CHARS ? (q.data ?? []) : [],
    searching: typing || q.isFetching,
    active: query.trim().length >= MIN_CHARS,
  };
}
