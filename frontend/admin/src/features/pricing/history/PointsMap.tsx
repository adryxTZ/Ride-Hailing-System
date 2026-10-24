"use client";
/**
 * PointsMap — click to place the pickup (A), click again for the drop-off (B);
 * a third click starts over. Draws the quoted road when one is given (an
 * encoded polyline). Follows the country picked in the top bar.
 */
import maplibregl from "maplibre-gl";
import { useEffect, useRef } from "react";
import { DEFAULT_CENTRE, dot, useMap } from "@/components/map/useMap";
import type { LatLng } from "./hooks";

const ROUTE = "sim-route";
const MARKER_PX = 26;
const MAP_ZOOM = 12;

interface Props {
  pickup?: LatLng;
  dropoff?: LatLng;
  onChange: (pickup?: LatLng, dropoff?: LatLng) => void;
  polyline?: string;
  label: string;
}

/** Google/OSRM encoded polyline (precision 5) → [lng, lat] pairs. */
function decode(line: string): [number, number][] {
  const out: [number, number][] = [];
  let i = 0;
  let lat = 0;
  let lng = 0;
  const next = () => {
    let shift = 0;
    let result = 0;
    let b: number;
    do {
      b = line.charCodeAt(i++) - 63;
      result |= (b & 0x1f) << shift;
      shift += 5;
    } while (b >= 0x20);
    return result & 1 ? ~(result >> 1) : result >> 1;
  };
  while (i < line.length) {
    lat += next();
    lng += next();
    out.push([lng / 1e5, lat / 1e5]);
  }
  return out;
}

function lettered(letter: string, colour: string, ink: string, label: string) {
  const el = dot(colour, MARKER_PX, label);
  el.textContent = letter;
  Object.assign(el.style, {
    display: "grid",
    placeItems: "center",
    color: `var(${ink})`,
    fontSize: "11px",
    fontWeight: "700",
  });
  return el;
}

export function PointsMap({ pickup, dropoff, onChange, polyline, label }: Props) {
  const { container, map } = useMap(MAP_ZOOM, DEFAULT_CENTRE);
  // The click handler reads the latest points without re-binding.
  const points = useRef({ pickup, dropoff });
  points.current = { pickup, dropoff };

  useEffect(() => {
    if (!map) return;
    const click = (e: maplibregl.MapMouseEvent) => {
      const p = { lat: e.lngLat.lat, lng: e.lngLat.lng };
      const { pickup: a, dropoff: b } = points.current;
      if (!a || b) onChange(p, undefined);
      else onChange(a, p);
    };
    map.on("click", click);
    map.getCanvas().style.cursor = "crosshair";
    return () => {
      map.off("click", click);
    };
  }, [map, onChange]);

  useEffect(() => {
    if (!map) return;
    const markers: maplibregl.Marker[] = [];
    if (pickup)
      markers.push(
        new maplibregl.Marker({ element: lettered("A", "--brand", "--brand-ink", "Pickup") })
          .setLngLat([pickup.lng, pickup.lat])
          .addTo(map),
      );
    if (dropoff)
      markers.push(
        new maplibregl.Marker({ element: lettered("B", "--ink", "--surface", "Drop-off") })
          .setLngLat([dropoff.lng, dropoff.lat])
          .addTo(map),
      );
    return () => markers.forEach((m) => m.remove());
  }, [map, pickup, dropoff]);

  useEffect(() => {
    if (!map || !polyline) return;
    const colour = getComputedStyle(document.documentElement).getPropertyValue("--chart-1").trim();
    map.addSource(ROUTE, {
      type: "geojson",
      data: {
        type: "Feature",
        properties: {},
        geometry: { type: "LineString", coordinates: decode(polyline) },
      },
    });
    map.addLayer({
      id: ROUTE,
      type: "line",
      source: ROUTE,
      layout: { "line-cap": "round" },
      paint: { "line-color": colour, "line-width": 4, "line-opacity": 0.7 },
    });
    return () => {
      if (!map.style) return; // the map is already gone
      if (map.getLayer(ROUTE)) map.removeLayer(ROUTE);
      if (map.getSource(ROUTE)) map.removeSource(ROUTE);
    };
  }, [map, polyline]);

  return (
    <div>
      <div
        ref={container}
        className="h-[360px] w-full overflow-hidden rounded-[10px] bg-surface-2"
        role="application"
        aria-label={label}
      />
      <p className="mt-2 text-[11px] text-muted">
        {!pickup
          ? "Click the map to place the pickup."
          : !dropoff
            ? "Click again to place the drop-off (optional)."
            : "Click again to start over."}
      </p>
    </div>
  );
}
