"use client";
/** Trip stops and driver, with a dashed stop connection (not a recorded road route). */
import { LocateFixed } from "lucide-react";
import maplibregl from "maplibre-gl";
import { useEffect } from "react";
import { DEFAULT_CENTRE, dot, useMap } from "./useMap";

interface Props {
  stops: { kind: string; lat: number; lng: number }[];
  driver?: { lat: number; lng: number };
  className?: string;
}

export function RouteMap({ stops, driver, className = "h-56 rounded-[10px]" }: Props) {
  const { container, map } = useMap(12, DEFAULT_CENTRE, false);
  function fitStops() {
    if (!map || !stops.length) return;
    const points = [...stops, ...(driver ? [driver] : [])].map((p) => [p.lng, p.lat] as [number, number]);
    const bounds = points.reduce((b, p) => b.extend(p), new maplibregl.LngLatBounds(points[0], points[0]));
    map.fitBounds(bounds, { padding: 55, maxZoom: 15, duration: 0 });
  }

  useEffect(() => {
    if (!map || stops.length === 0) return;
    const markers = stops.map((stop, index) => {
      const label = `${String.fromCharCode(65 + index)} · ${stop.kind === "PICKUP" ? "Pickup" : stop.kind === "DROPOFF" ? "Drop-off" : "Stop"}`;
      const element = dot(
        stop.kind === "PICKUP" ? "--brand" : stop.kind === "DROPOFF" ? "--ink" : "--muted",
        30,
        label,
      );
      element.textContent = String.fromCharCode(65 + index);
      Object.assign(element.style, {
        display: "grid",
        placeItems: "center",
        color: stop.kind === "DROPOFF" ? "var(--surface)" : "var(--brand-ink)",
        fontSize: "11px",
        fontWeight: "700",
        borderWidth: "3px",
      });
      return new maplibregl.Marker({ element }).setLngLat([stop.lng, stop.lat]).addTo(map);
    });
    if (driver)
      markers.push(
        new maplibregl.Marker({ element: dot("--chart-1", 13, "Driver") })
          .setLngLat([driver.lng, driver.lat])
          .addTo(map),
      );
    if (stops.length > 1) {
      const brand = getComputedStyle(document.documentElement).getPropertyValue("--brand").trim();
      map.addSource("trip-stop-connection", {
        type: "geojson",
        data: {
          type: "Feature",
          properties: {},
          geometry: { type: "LineString", coordinates: stops.map((s) => [s.lng, s.lat]) },
        },
      });
      map.addLayer({
        id: "trip-stop-connection-halo",
        type: "line",
        source: "trip-stop-connection",
        paint: { "line-color": "#ffffff", "line-width": 6, "line-opacity": 0.75 },
      });
      map.addLayer({
        id: "trip-stop-connection",
        type: "line",
        source: "trip-stop-connection",
        paint: { "line-color": brand, "line-width": 2.5, "line-dasharray": [2, 2], "line-opacity": 0.8 },
      });
    }
    fitStops();
    map.on("resize", fitStops);
    return () => {
      map.off("resize", fitStops);
      markers.forEach((m) => m.remove());
      // useMap may have already destroyed the style during page navigation.
      if (!map.style) return;
      if (map.getLayer("trip-stop-connection")) map.removeLayer("trip-stop-connection");
      if (map.getLayer("trip-stop-connection-halo")) map.removeLayer("trip-stop-connection-halo");
      if (map.getSource("trip-stop-connection")) map.removeSource("trip-stop-connection");
    };
  }, [map, stops, driver]);
  return (
    <div className={`relative w-full overflow-hidden bg-surface-2 ${className}`}>
      <div
        ref={container}
        className="h-full w-full"
        role="region"
        aria-label="Interactive map of trip stops"
      />
      <button
        type="button"
        onClick={fitStops}
        disabled={!map || !stops.length}
        aria-label="Fit map to trip stops"
        title="Fit map to trip stops"
        className="absolute right-3 top-3 grid size-9 place-items-center rounded-lg border border-line bg-surface text-ink shadow-sm transition-colors hover:bg-surface-2 disabled:opacity-40"
      >
        <LocateFixed size={17} aria-hidden="true" />
      </button>
    </div>
  );
}
