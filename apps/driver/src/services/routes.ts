/** routes service — road route between two points (cached server-side). Pure data access. */
import type { components } from "@platform/api-client";
import { api } from "@/services/api";

/** One manoeuvre of a road route (turn, roundabout, arrival…). */
export type RouteStep = components["schemas"]["RouteStep"];

export interface RoadRoute {
  distance_m: number;
  duration_s: number;
  polyline?: string;
  provider: string;
  /** Turn-by-turn manoeuvres from the start to the end, in order. */
  steps?: RouteStep[];
}

type LatLng = { lat: number; lng: number };

const ll = (p: LatLng) => `${p.lat},${p.lng}`;

/** Road route from → (via stops, in order) → to. */
export async function fetchRoute(
  from: LatLng,
  to: LatLng,
  via: LatLng[] = [],
  signal?: AbortSignal,
): Promise<RoadRoute> {
  const { data, error } = await api.GET("/routes", {
    params: { query: { from: ll(from), to: ll(to), via: via.length ? via.map(ll).join("|") : undefined } },
    signal,
  });
  if (error || !data) throw new Error("route failed");
  return data;
}
