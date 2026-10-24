"use client";
/**
 * GeofenceMap — every geofence of the country (districts, zones, places as
 * circles), the selected one highlighted, and the shape being drawn: click to
 * add a corner (or two opposite corners with the rectangle tool), drag a corner
 * to move it, or click to place a centre (place). Colours are tokens.
 */
import maplibregl from "maplibre-gl";
import { useEffect } from "react";
import { DEFAULT_CENTRE, useMap } from "@/components/map/useMap";
import type { Schemas } from "@/lib/api";
import { boundsOf, circleRing, featureOf } from "./shapes";

const FENCES = "geofences";
const DRAFT = "geofence-draft";
const FIT_PADDING = 40;
const MAX_ZOOM = 15;

export type DrawMode = "off" | "polygon" | "place";

/** How a polygon is drawn: corner by corner, or a rectangle from two opposite corners. */
export type PolygonTool = "corners" | "rectangle";

export interface Draft {
  points: [number, number][];
  tool?: PolygonTool;
  center?: { lat: number; lng: number };
  radiusM: number;
}

const token = (v: string) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();

export function GeofenceMap({
  fences,
  selectedId,
  mode,
  draft,
  onDraft,
  onSelect,
  height = "h-[560px]",
}: {
  fences: Schemas["Geofence"][];
  selectedId?: string;
  mode: DrawMode;
  draft: Draft;
  onDraft: (d: Draft) => void;
  onSelect?: (id: string) => void;
  height?: string;
}) {
  const { container, map } = useMap(11, DEFAULT_CENTRE, fences.length === 0);
  useEffect(() => {
    if (!map) return;
    ensureLayers(map);
    const data: GeoJSON.FeatureCollection = {
      type: "FeatureCollection",
      features: fences.map((f) => featureOf(f, f.id === selectedId)).filter((f): f is GeoJSON.Feature => !!f),
    };
    (map.getSource(FENCES) as maplibregl.GeoJSONSource).setData(data);
  }, [map, fences, selectedId]);
  useEffect(() => {
    if (!map) return;
    const b = boundsOf(selectedId ? fences.filter((f) => f.id === selectedId) : fences);
    if (b)
      map.fitBounds(
        [
          [b[0], b[1]],
          [b[2], b[3]],
        ],
        { padding: FIT_PADDING, maxZoom: MAX_ZOOM, duration: 0 },
      );
  }, [map, selectedId, fences.length]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!map) return;
    ensureLayers(map);
    (map.getSource(DRAFT) as maplibregl.GeoJSONSource).setData(draftFeatures(draft));
  }, [map, draft]);
  useEffect(() => {
    if (!map) return;
    const onClick = (e: maplibregl.MapMouseEvent) => {
      if (mode === "polygon") onDraft({ ...draft, points: nextPoints(draft, [e.lngLat.lng, e.lngLat.lat]) });
      else if (mode === "place") onDraft({ ...draft, center: { lat: e.lngLat.lat, lng: e.lngLat.lng } });
      else if (onSelect) {
        const hit = map.queryRenderedFeatures(e.point, { layers: [`${FENCES}-fill`] })[0];
        if (hit?.properties?.id) onSelect(String(hit.properties.id));
      }
    };
    map.on("click", onClick);
    map.getCanvas().style.cursor = mode === "off" ? "" : "crosshair";
    return () => {
      map.off("click", onClick);
    };
  }, [map, mode, draft, onDraft, onSelect]);
  useCornerDrag(map, mode === "polygon", draft, onDraft);
  return (
    <div
      ref={container}
      className={`${height} w-full overflow-hidden rounded-[10px] bg-surface-2`}
      role="img"
      aria-label="Map of geofences"
    />
  );
}

/** The draft after a click: one more corner, or with the rectangle tool the
 *  first corner and then the whole rectangle (a third click starts again). */
function nextPoints(d: Draft, p: [number, number]): [number, number][] {
  if (d.tool !== "rectangle") return [...d.points, p];
  if (d.points.length !== 1) return [p];
  const [a] = d.points;
  return [a, [p[0], a[1]], p, [a[0], p[1]]];
}

/** Drag a corner of the draft polygon to move it (the map does not pan meanwhile). */
function useCornerDrag(
  map: maplibregl.Map | undefined,
  on: boolean,
  draft: Draft,
  onDraft: (d: Draft) => void,
) {
  useEffect(() => {
    if (!map || !on) return;
    const layer = `${DRAFT}-corner`;
    let dragging: number | undefined;
    const down = (e: maplibregl.MapLayerMouseEvent) => {
      const idx = e.features?.[0]?.properties?.idx;
      if (typeof idx !== "number") return;
      e.preventDefault();
      dragging = idx;
      map.dragPan.disable();
    };
    const move = (e: maplibregl.MapMouseEvent) => {
      if (dragging === undefined) return;
      const points = draft.points.map((p, i) =>
        i === dragging ? ([e.lngLat.lng, e.lngLat.lat] as [number, number]) : p,
      );
      onDraft({ ...draft, points });
    };
    const up = () => {
      if (dragging === undefined) return;
      dragging = undefined;
      map.dragPan.enable();
    };
    const hover = () => (map.getCanvas().style.cursor = "move");
    const leave = () => (map.getCanvas().style.cursor = "crosshair");
    map.on("mousedown", layer, down);
    map.on("mouseenter", layer, hover);
    map.on("mouseleave", layer, leave);
    map.on("mousemove", move);
    map.on("mouseup", up);
    return () => {
      map.off("mousedown", layer, down);
      map.off("mouseenter", layer, hover);
      map.off("mouseleave", layer, leave);
      map.off("mousemove", move);
      map.off("mouseup", up);
      map.dragPan.enable();
    };
  }, [map, on, draft, onDraft]);
}

/** The draft polygon (line, corners, closing fill) or the place's circle. */
function draftFeatures(d: Draft): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];
  if (d.points.length > 0) {
    features.push({
      type: "Feature",
      properties: { part: "line" },
      geometry: {
        type: "LineString",
        coordinates: d.points.length > 2 ? [...d.points, d.points[0]] : d.points,
      },
    });
    d.points.forEach((p, idx) =>
      features.push({
        type: "Feature",
        properties: { part: "corner", idx },
        geometry: { type: "Point", coordinates: p },
      }),
    );
  }
  if (d.center) {
    features.push({
      type: "Feature",
      properties: { part: "circle" },
      geometry: { type: "Polygon", coordinates: [circleRing(d.center.lat, d.center.lng, d.radiusM)] },
    });
    features.push({
      type: "Feature",
      properties: { part: "corner" },
      geometry: { type: "Point", coordinates: [d.center.lng, d.center.lat] },
    });
  }
  return { type: "FeatureCollection", features };
}

/** Adds the sources and layers once per map. */
function ensureLayers(map: maplibregl.Map) {
  if (map.getSource(FENCES)) return;
  const brand = token("--brand"),
    chart = token("--chart-1"),
    accent = token("--chart-2"),
    muted = token("--muted");
  const empty: GeoJSON.FeatureCollection = { type: "FeatureCollection", features: [] };
  map.addSource(FENCES, { type: "geojson", data: empty });
  map.addSource(DRAFT, { type: "geojson", data: empty });
  map.addLayer({
    id: `${FENCES}-fill`,
    type: "fill",
    source: FENCES,
    paint: {
      "fill-color": ["case", ["get", "selected"], brand, ["==", ["get", "kind"], "PLACE"], accent, chart],
      "fill-opacity": ["case", ["get", "inactive"], 0.05, ["get", "selected"], 0.3, 0.15],
    },
  });
  map.addLayer({
    id: `${FENCES}-line`,
    type: "line",
    source: FENCES,
    paint: {
      "line-color": ["case", ["get", "selected"], brand, ["get", "inactive"], muted, chart],
      "line-width": ["case", ["get", "selected"], 2.5, 1.5],
    },
  });
  map.addLayer({
    id: `${DRAFT}-fill`,
    type: "fill",
    source: DRAFT,
    filter: ["==", ["get", "part"], "circle"],
    paint: { "fill-color": brand, "fill-opacity": 0.2 },
  });
  map.addLayer({
    id: `${DRAFT}-line`,
    type: "line",
    source: DRAFT,
    filter: ["==", ["get", "part"], "line"],
    paint: { "line-color": brand, "line-width": 2, "line-dasharray": [2, 1] },
  });
  map.addLayer({
    id: `${DRAFT}-corner`,
    type: "circle",
    source: DRAFT,
    filter: ["==", ["get", "part"], "corner"],
    paint: {
      "circle-radius": 5,
      "circle-color": brand,
      "circle-stroke-color": token("--surface"),
      "circle-stroke-width": 2,
    },
  });
}
