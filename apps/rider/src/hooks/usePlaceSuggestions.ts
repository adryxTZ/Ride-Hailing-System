/**
 * usePlaceSuggestions — saved places plus recent drop-offs, de-duplicated and
 * filtered by the search text, each with its distance from `origin`.
 */
import { useMemo } from "react";
import { usePlaces } from "@/hooks/useProfile";
import { useTripHistory } from "@/hooks/useTrips";
import { distanceKm } from "@/models/geo";
import type { Point } from "@/services/trips";

export interface Suggestion {
  key: string;
  title: string;
  subtitle?: string;
  icon: string;
  point: Point;
  distanceKm?: number;
}

const KIND_ICON: Record<string, string> = { HOME: "home-outline", WORK: "briefcase-outline" };

export function usePlaceSuggestions(
  query: string,
  origin?: Point,
): { items: Suggestion[]; loading: boolean } {
  const { data: places, isLoading: placesLoading } = usePlaces();
  const { data: history, isLoading: historyLoading } = useTripHistory();
  const items = useMemo(() => {
    const out: Suggestion[] = [];
    const seen = new Set<string>();
    for (const p of places ?? []) {
      seen.add(p.label.toLowerCase());
      out.push({
        key: `p-${p.id}`,
        title: p.label,
        subtitle: p.address,
        icon: KIND_ICON[p.kind] ?? "map-marker-outline",
        point: { lat: p.lat, lng: p.lng },
      });
    }
    for (const t of history ?? []) {
      const drop = t.stops.find((s) => s.kind === "DROPOFF");
      const title = drop?.address;
      if (!drop || !title || seen.has(title.toLowerCase())) continue;
      seen.add(title.toLowerCase());
      out.push({ key: `t-${t.id}`, title, icon: "clock-outline", point: drop.location });
    }
    const q = query.trim().toLowerCase();
    return out
      .filter((s) => !q || s.title.toLowerCase().includes(q) || s.subtitle?.toLowerCase().includes(q))
      .map((s) => ({ ...s, distanceKm: origin ? distanceKm(origin, s.point) : undefined }));
  }, [places, history, query, origin]);
  return { items, loading: placesLoading || historyLoading };
}
