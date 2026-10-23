"use client";
/**
 * PersonMap — where a person goes: last known location (brand), likely home
 * (chart-2), saved places (chart-3) and their frequent routes as lines whose
 * weight follows how often they were travelled. Framed to fit; colours are tokens.
 */
import maplibregl from "maplibre-gl";
import { useEffect } from "react";
import { DEFAULT_CENTRE, dot, useMap } from "@/components/map/useMap";
import type { Schemas } from "@/lib/api";

const ROUTES_SOURCE = "person-routes";
const MIN_LINE_PX = 2;
const MAX_LINE_PX = 7;
const FIT_PADDING = 48;
const MAX_ZOOM = 14;

type Places = Schemas["PersonPlaces"];

/** The routes as one GeoJSON layer: a straight line per pickup → drop-off pair. */
function routeFeatures(routes: Places["routes"]): GeoJSON.FeatureCollection {
  const top = Math.max(1, ...routes.map((r) => r.trips));
  return {
    type: "FeatureCollection",
    features: routes.map((r) => ({
      type: "Feature",
      properties: { w: MIN_LINE_PX + ((MAX_LINE_PX - MIN_LINE_PX) * r.trips) / top },
      geometry: {
        type: "LineString",
        coordinates: [
          [r.from.lng, r.from.lat],
          [r.to.lng, r.to.lat],
        ],
      },
    })),
  };
}

function addRoutes(map: maplibregl.Map, routes: Places["routes"]) {
  const brand = getComputedStyle(document.documentElement).getPropertyValue("--chart-1").trim();
  map.addSource(ROUTES_SOURCE, { type: "geojson", data: routeFeatures(routes) });
  map.addLayer({
    id: ROUTES_SOURCE,
    type: "line",
    source: ROUTES_SOURCE,
    layout: { "line-cap": "round" },
    paint: { "line-color": brand, "line-opacity": 0.55, "line-width": ["get", "w"] },
  });
}

export function PersonMap({ places }: { places: Places }) {
  const { container, map } = useMap(12, DEFAULT_CENTRE, false);
  useEffect(() => {
    if (!map) return;
    const pins: { lat: number; lng: number; colour: string; size: number; label: string }[] = [];
    for (const s of places.saved)
      pins.push({ lat: s.lat, lng: s.lng, colour: "--chart-3", size: 12, label: `Saved: ${s.label}` });
    if (places.likely_home)
      pins.push({ ...places.likely_home, colour: "--chart-2", size: 14, label: "Likely home" });
    if (places.last_location)
      pins.push({ ...places.last_location, colour: "--brand", size: 18, label: "Last known location" });
    const markers = pins.map((p) =>
      new maplibregl.Marker({ element: dot(p.colour, p.size, p.label) }).setLngLat([p.lng, p.lat]).addTo(map),
    );
    addRoutes(map, places.routes);
    const pts = [
      ...pins.map((p) => [p.lng, p.lat] as [number, number]),
      ...places.routes.flatMap((r) => [
        [r.from.lng, r.from.lat] as [number, number],
        [r.to.lng, r.to.lat] as [number, number],
      ]),
    ];
    if (pts.length)
      map.fitBounds(
        pts.reduce((b, p) => b.extend(p), new maplibregl.LngLatBounds(pts[0], pts[0])),
        { padding: FIT_PADDING, maxZoom: MAX_ZOOM, duration: 0 },
      );
    return () => {
      markers.forEach((m) => m.remove());
      // When the page leaves, useMap has already removed the map (and its style).
      if (!map.style) return;
      if (map.getLayer(ROUTES_SOURCE)) map.removeLayer(ROUTES_SOURCE);
      if (map.getSource(ROUTES_SOURCE)) map.removeSource(ROUTES_SOURCE);
    };
  }, [map, places]);
  return (
    <div>
      <div
        ref={container}
        className="h-[340px] w-full overflow-hidden rounded-[10px] bg-surface-2"
        role="img"
        aria-label="Map of the person's last location, likely home, saved places and frequent routes"
      />
      <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
        <Legend colour="--brand" label="Last known location" />
        <Legend colour="--chart-2" label="Likely home" />
        <Legend colour="--chart-3" label="Saved places" />
        <Legend colour="--chart-1" label="Frequent routes" />
      </ul>
    </div>
  );
}

function Legend({ colour, label }: { colour: string; label: string }) {
  return (
    <li className="inline-flex items-center gap-1.5">
      <span className="size-2.5 rounded-full" style={{ background: `var(${colour})` }} aria-hidden="true" />
      {label}
    </li>
  );
}
