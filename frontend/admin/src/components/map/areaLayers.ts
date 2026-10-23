"use client";
/** areaLayers — draws operating areas (their H3 cells) on a MapLibre map: fill, outline and name labels. */
import { cellToBoundary, cellToLatLng, uncompactCells } from "h3-js";
import type maplibregl from "maplibre-gl";

const SOURCE = "areas";
const CHART_VARS = ["--chart-1", "--chart-2", "--chart-3", "--chart-4", "--chart-5"];

export interface AreaShape {
  id: string;
  name: string;
  status: string;
  cells: string[]; // compacted H3 cells
}

/** Colour slot for an area, stable by position in the list. */
export function areaColour(index: number): string {
  return getComputedStyle(document.documentElement)
    .getPropertyValue(CHART_VARS[index % CHART_VARS.length])
    .trim();
}

/** GeoJSON for every cell of every area (one feature per res-7 cell). */
export function areaFeatures(areas: AreaShape[], selectedId?: string) {
  const features: GeoJSON.Feature[] = [];
  areas.forEach((a, i) => {
    const fine = uncompactCells(a.cells, 7);
    for (const cell of fine) {
      features.push({
        type: "Feature",
        properties: { cell, area: a.id, colour: areaColour(i), locked: !!selectedId && selectedId !== a.id },
        geometry: { type: "Polygon", coordinates: [cellToBoundary(cell, true)] },
      });
    }
  });
  return { type: "FeatureCollection" as const, features };
}

/** Label points: the centre cell of each area. */
export function areaLabels(areas: AreaShape[]) {
  return {
    type: "FeatureCollection" as const,
    features: areas
      .filter((a) => a.cells.length)
      .map((a) => {
        const [lat, lng] = cellToLatLng(a.cells[0]);
        return {
          type: "Feature" as const,
          properties: { name: a.name },
          geometry: { type: "Point" as const, coordinates: [lng, lat] },
        };
      }),
  };
}

/** Adds or updates the area layers. Call again whenever areas or the selection change. */
export function drawAreas(map: maplibregl.Map, areas: AreaShape[], selectedId?: string) {
  const data = areaFeatures(areas, selectedId);
  const labels = areaLabels(areas);
  const src = map.getSource(SOURCE) as maplibregl.GeoJSONSource | undefined;
  if (src) {
    src.setData(data);
    (map.getSource(`${SOURCE}-labels`) as maplibregl.GeoJSONSource).setData(labels);
    return;
  }
  map.addSource(SOURCE, { type: "geojson", data });
  map.addSource(`${SOURCE}-labels`, { type: "geojson", data: labels });
  map.addLayer({
    id: `${SOURCE}-fill`,
    type: "fill",
    source: SOURCE,
    paint: { "fill-color": ["get", "colour"], "fill-opacity": ["case", ["get", "locked"], 0.12, 0.28] },
  });
  map.addLayer({
    id: `${SOURCE}-line`,
    type: "line",
    source: SOURCE,
    paint: { "line-color": ["get", "colour"], "line-opacity": 0.5, "line-width": 0.6 },
  });
  map.addLayer({
    id: `${SOURCE}-label`,
    type: "symbol",
    source: `${SOURCE}-labels`,
    layout: { "text-field": ["get", "name"], "text-size": 13, "text-font": ["Noto Sans Bold"] },
    paint: {
      "text-color": getComputedStyle(document.documentElement).getPropertyValue("--ink").trim(),
      "text-halo-color": getComputedStyle(document.documentElement).getPropertyValue("--surface").trim(),
      "text-halo-width": 1.5,
    },
  });
}
