/** Turns component props into the plain state object the map page understands. */
import type { MapCanvasProps, MapState } from "./types";

export function toState(p: MapCanvasProps): MapState {
  return {
    centre: p.centre,
    padding: {
      top: p.padding.top,
      bottom: p.padding.bottom,
      left: p.padding.left ?? 0,
      right: p.padding.right ?? 0,
    },
    colors: p.colors,
    markers: p.markers ?? [],
    route: p.route ?? [],
    fit: p.fit ?? [],
    zoom: p.zoom,
    recenterKey: p.recenterKey ?? 0,
  };
}

/** Messages the page sends back. */
export type PageMessage =
  | { type: "ready" }
  | { type: "centre"; lat: number; lng: number }
  | { type: "error"; message: string };

export function parseMessage(raw: unknown): PageMessage | undefined {
  try {
    return typeof raw === "string" ? (JSON.parse(raw) as PageMessage) : undefined;
  } catch {
    return undefined;
  }
}

export const DEFAULT_ZOOM = 14.5;
