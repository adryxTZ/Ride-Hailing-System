"use client";
/**
 * useCellPainter — click or drag on the map to add or remove res-7 cells of
 * the area being edited. Keeps the unsaved set; the screen saves it with a reason.
 */
import { cellToBoundary, latLngToCell, polygonToCells } from "h3-js";
import type maplibregl from "maplibre-gl";
import { useCallback, useEffect, useRef, useState } from "react";

export const PAINT_RES = 7;
/** Below this zoom cells are too small to pick one by one. */
export const PAINT_MIN_ZOOM = 8;
const SOURCE = "paint";

export type PaintMode = "paint" | "erase" | "polygon" | "off";

export function useCellPainter(map: maplibregl.Map | undefined, initial: Set<string>, mode: PaintMode) {
  const [cells, setCells] = useState<Set<string>>(initial);
  const [ring, setRing] = useState<[number, number][]>([]);
  const dragging = useRef(false);
  const modeRef = useRef(mode);
  modeRef.current = mode;

  const apply = useCallback((cell: string) => {
    setCells((prev) => {
      const next = new Set(prev);
      if (modeRef.current === "erase") next.delete(cell);
      else next.add(cell);
      return next;
    });
  }, []);

  useEffect(() => {
    if (!map) return;
    const cellAt = (e: maplibregl.MapMouseEvent) => latLngToCell(e.lngLat.lat, e.lngLat.lng, PAINT_RES);
    const onDown = (e: maplibregl.MapMouseEvent) => {
      if (modeRef.current === "off" || map.getZoom() < PAINT_MIN_ZOOM) return;
      if (modeRef.current === "polygon") {
        setRing((r) => [...r, [e.lngLat.lng, e.lngLat.lat]]);
        return;
      }
      dragging.current = true;
      map.dragPan.disable();
      apply(cellAt(e));
    };
    const onMove = (e: maplibregl.MapMouseEvent) => {
      if (dragging.current) apply(cellAt(e));
    };
    const onUp = () => {
      dragging.current = false;
      map.dragPan.enable();
    };
    map.on("mousedown", onDown);
    map.on("mousemove", onMove);
    map.on("mouseup", onUp);
    return () => {
      map.off("mousedown", onDown);
      map.off("mousemove", onMove);
      map.off("mouseup", onUp);
      map.dragPan.enable();
    };
  }, [map, apply]);

  // Draw the unsaved cells and the polygon in progress.
  useEffect(() => {
    if (!map) return;
    const css = getComputedStyle(document.documentElement);
    const features: GeoJSON.Feature[] = [...cells].map((cell) => ({
      type: "Feature",
      properties: {},
      geometry: { type: "Polygon", coordinates: [cellToBoundary(cell, true)] },
    }));
    const data = { type: "FeatureCollection" as const, features };
    const line = {
      type: "Feature" as const,
      properties: {},
      geometry: { type: "LineString" as const, coordinates: ring },
    };
    const src = map.getSource(SOURCE) as maplibregl.GeoJSONSource | undefined;
    if (src) {
      src.setData(data);
      (map.getSource(`${SOURCE}-ring`) as maplibregl.GeoJSONSource).setData(line);
      return;
    }
    map.addSource(SOURCE, { type: "geojson", data });
    map.addSource(`${SOURCE}-ring`, { type: "geojson", data: line });
    map.addLayer({
      id: `${SOURCE}-fill`,
      type: "fill",
      source: SOURCE,
      paint: { "fill-color": css.getPropertyValue("--brand").trim(), "fill-opacity": 0.35 },
    });
    map.addLayer({
      id: `${SOURCE}-line`,
      type: "line",
      source: SOURCE,
      paint: { "line-color": css.getPropertyValue("--brand").trim(), "line-width": 1 },
    });
    map.addLayer({
      id: `${SOURCE}-ring`,
      type: "line",
      source: `${SOURCE}-ring`,
      paint: {
        "line-color": css.getPropertyValue("--ink").trim(),
        "line-width": 2,
        "line-dasharray": [2, 2],
      },
    });
  }, [map, cells, ring]);

  /** Closes the polygon: its cells are added (or removed in erase mode). */
  const closePolygon = useCallback(
    (remove: boolean) => {
      if (ring.length < 3) return;
      const found = polygonToCells(
        [...ring, ring[0]].map(([lng, lat]) => [lat, lng]),
        PAINT_RES,
      );
      setCells((prev) => {
        const next = new Set(prev);
        for (const c of found) remove ? next.delete(c) : next.add(c);
        return next;
      });
      setRing([]);
    },
    [ring],
  );

  const reset = useCallback((to: Set<string>) => {
    setCells(to);
    setRing([]);
  }, []);

  return { cells, ring, closePolygon, reset, setCells };
}
