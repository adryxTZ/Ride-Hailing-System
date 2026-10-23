"use client";
/**
 * useHexGrid — draws the H3 grid over the visible map (the cells dispatch and
 * surge work in) and tints each cell by how many free drivers and waiting riders
 * it holds. Cells are recomputed when the map stops moving.
 */
import { cellToBoundary, cellToParent, getResolution, latLngToCell, polygonToCells } from "h3-js";
import type maplibregl from "maplibre-gl";
import { useEffect } from "react";

/** Grid resolution: res 8 cells are ~0.7 km across, readable at city zoom. */
export const GRID_RES = 8;
/** Below this zoom the grid is too dense to read, so it is hidden. */
const MIN_ZOOM = 10;
const SOURCE = "hex-grid";

interface Point {
  lat: number;
  lng: number;
}

export interface GridInput {
  drivers: { location: Point; h3?: string }[];
  requests: { pickup: Point }[];
}

/** The cell at grid resolution for a point, or from a finer presence cell. */
function gridCell(p: Point, h3?: string): string {
  if (h3 && getResolution(h3) >= GRID_RES) return cellToParent(h3, GRID_RES);
  return latLngToCell(p.lat, p.lng, GRID_RES);
}

/** Cells covering the current view (plus one ring beyond the edges). */
function viewportCells(map: maplibregl.Map): string[] {
  const b = map.getBounds();
  const ring = [
    [b.getNorth(), b.getWest()],
    [b.getNorth(), b.getEast()],
    [b.getSouth(), b.getEast()],
    [b.getSouth(), b.getWest()],
  ];
  return polygonToCells(ring, GRID_RES);
}

export function useHexGrid(map: maplibregl.Map | undefined, input: GridInput, enabled: boolean) {
  useEffect(() => {
    if (!map) return;
    const css = getComputedStyle(document.documentElement);
    const line = css.getPropertyValue("--muted").trim();
    const supply = css.getPropertyValue("--chart-1").trim();
    const demand = css.getPropertyValue("--critical").trim();

    const render = () => {
      const drivers = new Map<string, number>();
      for (const d of input.drivers) {
        const c = gridCell(d.location, d.h3);
        drivers.set(c, (drivers.get(c) ?? 0) + 1);
      }
      const waiting = new Map<string, number>();
      for (const r of input.requests) {
        const c = gridCell(r.pickup);
        waiting.set(c, (waiting.get(c) ?? 0) + 1);
      }
      const cells = map.getZoom() >= MIN_ZOOM && enabled ? viewportCells(map) : [];
      const features = cells.map((cell) => ({
        type: "Feature" as const,
        properties: { cell, drivers: drivers.get(cell) ?? 0, waiting: waiting.get(cell) ?? 0 },
        geometry: { type: "Polygon" as const, coordinates: [cellToBoundary(cell, true)] },
      }));
      const data = { type: "FeatureCollection" as const, features };
      const src = map.getSource(SOURCE) as maplibregl.GeoJSONSource | undefined;
      if (src) {
        src.setData(data);
        return;
      }
      map.addSource(SOURCE, { type: "geojson", data });
      map.addLayer({
        id: `${SOURCE}-fill`,
        type: "fill",
        source: SOURCE,
        paint: {
          // Waiting riders tint red, free drivers tint blue; empty cells stay clear.
          "fill-color": ["case", [">", ["get", "waiting"], 0], demand, supply],
          "fill-opacity": [
            "interpolate",
            ["linear"],
            ["+", ["get", "drivers"], ["*", 2, ["get", "waiting"]]],
            0,
            0,
            1,
            0.18,
            6,
            0.45,
          ],
        },
      });
      map.addLayer({
        id: `${SOURCE}-line`,
        type: "line",
        source: SOURCE,
        paint: { "line-color": line, "line-opacity": 0.35, "line-width": 0.8 },
      });
    };

    render();
    map.on("moveend", render);
    return () => {
      map.off("moveend", render);
    };
  }, [map, input, enabled]);
}
