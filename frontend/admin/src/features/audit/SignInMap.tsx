"use client";
/** SignInMap — where staff signed in from: one dot per located sign-in (failed ones in red), fitted to them. */
import maplibregl from "maplibre-gl";
import { useEffect } from "react";
import { DEFAULT_CENTRE, dot, useMap } from "@/components/map/useMap";
import type { SignIn } from "./signIns";

/** Zoom used when there is a single point to show. */
const SINGLE_POINT_ZOOM = 13;
const FIT_PADDING = 48;

export function SignInMap({
  signIns,
  selectedId,
  onSelect,
}: {
  signIns: SignIn[];
  selectedId?: number;
  onSelect: (id: number) => void;
}) {
  const { container, map } = useMap(5, DEFAULT_CENTRE, false);
  const located = signIns.filter((s) => s.location);

  useEffect(() => {
    if (!map) return;
    const markers = located.map((s) => {
      const el = dot(
        s.failed ? "--critical" : s.id === selectedId ? "--brand" : "--chart-1",
        s.id === selectedId ? 18 : 13,
        `${s.who} · ${s.when} · ${s.device}`,
      );
      el.style.cursor = "pointer";
      el.addEventListener("click", () => onSelect(s.id));
      return new maplibregl.Marker({ element: el }).setLngLat([s.location!.lng, s.location!.lat]).addTo(map);
    });
    return () => markers.forEach((m) => m.remove());
  }, [map, located, selectedId, onSelect]);

  // Frame every point once they arrive; zoom to the selected one when picked.
  useEffect(() => {
    if (!map || !located.length) return;
    const sel = located.find((s) => s.id === selectedId);
    if (sel) {
      map.easeTo({
        center: [sel.location!.lng, sel.location!.lat],
        zoom: Math.max(map.getZoom(), SINGLE_POINT_ZOOM),
        duration: 400,
      });
      return;
    }
    if (located.length === 1) {
      map.jumpTo({ center: [located[0].location!.lng, located[0].location!.lat], zoom: SINGLE_POINT_ZOOM });
      return;
    }
    const b = new maplibregl.LngLatBounds();
    for (const s of located) b.extend([s.location!.lng, s.location!.lat]);
    map.fitBounds(b, { padding: FIT_PADDING, maxZoom: SINGLE_POINT_ZOOM, duration: 0 });
  }, [map, located.length, selectedId]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div
      ref={container}
      className="h-[420px] w-full overflow-hidden rounded-[10px] bg-surface-2"
      role="img"
      aria-label="Map of staff sign-in locations"
    />
  );
}
