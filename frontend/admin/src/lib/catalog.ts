"use client";
/** catalog — ride type names from the service catalogue ("xl" → "XL"), cached. */
import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";

const STALE_MS = 10 * 60_000;

/** Readable name for keys the catalogue no longer lists ("septic_7k" → "Septic 7k"). */
const humanise = (k: string) => k.replace(/_/g, " ").replace(/^\w/, (c) => c.toUpperCase());

function useServices() {
  return useQuery({
    queryKey: ["services"],
    queryFn: async () => unwrap(await api.GET("/services")).data,
    staleTime: STALE_MS,
  });
}

/** Every vehicle class key in the catalogue, once each, in catalogue order. */
export function useClassKeys(): string[] {
  const { data } = useServices();
  return [...new Set((data ?? []).flatMap((s) => s.vehicle_classes.map((c) => c.key)))];
}

export function useClassName() {
  const { data } = useServices();
  const names = new Map<string, string>();
  for (const s of data ?? []) for (const c of s.vehicle_classes) names.set(c.key, c.name.en ?? c.key);
  return (key: string) => names.get(key) ?? humanise(key);
}

/** "Ride" or "Package" for a service key. */
export const serviceLabel = (key: string) => (key === "parcel" ? "Package" : "Ride");
