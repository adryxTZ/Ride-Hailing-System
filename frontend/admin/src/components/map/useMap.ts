"use client";
/** useMap — one MapLibre map in a container (OpenFreeMap Positron tiles), removed on unmount. */
import maplibregl, { type Map as MlMap } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { useEffect, useRef, useState } from "react";
import { useAreas, useCountries, useScope } from "@/lib/scope";

export const MAP_STYLE = "https://tiles.openfreemap.org/styles/positron";
/** Where the map opens before the country is known (replaced as soon as it is). */
export const DEFAULT_CENTRE: [number, number] = [39.2083, -6.7924];
const FOLLOW_MS = 600;
const FIT_PADDING = 24;

/**
 * One map per container. By default it opens on, and follows, the country (and
 * region) picked in the top bar; pages that frame their own data (a trip route,
 * sign-ins, the regions editor) pass follow=false.
 */
export function useMap(zoom = 11, centre: [number, number] = DEFAULT_CENTRE, follow = true) {
  const container = useRef<HTMLDivElement>(null);
  const [map, setMap] = useState<MlMap>();
  const { scope } = useScope();
  const { data: countries } = useCountries();
  const { data: areas } = useAreas(scope.country);
  const country = countries?.find((c) => c.code === scope.country);
  const area = areas?.find((a) => a.id === scope.areaId);

  useEffect(() => {
    if (!container.current) return;
    const m = new maplibregl.Map({
      container: container.current,
      style: MAP_STYLE,
      center: centre,
      zoom,
      attributionControl: { compact: true },
    });
    m.addControl(new maplibregl.NavigationControl({ showCompass: false }), "bottom-right");
    m.on("load", () => setMap(m));
    return () => m.remove();
  }, [zoom]); // eslint-disable-line react-hooks/exhaustive-deps

  // Follow the picker: the selected region's outline, else the country's home view.
  useEffect(() => {
    if (!map || !follow || !country) return;
    if (area && area.cell_count > 0) {
      const [w, s, e, n] = area.bbox;
      map.fitBounds(
        [
          [w, s],
          [e, n],
        ],
        { padding: FIT_PADDING, duration: FOLLOW_MS },
      );
    } else {
      map.easeTo({ center: [country.map.lng, country.map.lat], zoom: country.map.zoom, duration: FOLLOW_MS });
    }
  }, [map, follow, country?.code, area?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  return { container, map };
}

/** A round dot marker element (colours from tokens, so no literals here). */
export function dot(colourVar: string, size = 14, label?: string): HTMLElement {
  const el = document.createElement("div");
  el.style.width = `${size}px`;
  el.style.height = `${size}px`;
  el.style.borderRadius = "50%";
  el.style.background = `var(${colourVar})`;
  el.style.border = "2px solid var(--surface)";
  el.style.boxShadow = "0 1px 4px color-mix(in srgb, var(--ink) 30%, transparent)";
  if (label) {
    el.title = label;
    el.setAttribute("aria-label", label);
  }
  return el;
}
