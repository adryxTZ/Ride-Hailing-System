/**
 * @platform/maps — one map component for both apps.
 * MapCanvas renders MapLibre GL (OpenFreeMap vector tiles, no API key) in a
 * WebView on phones and an iframe on web. Screens pass plain data (markers,
 * route, camera intent) and theme colours; they never touch the map engine.
 */
export { DEFAULT_STYLE } from "./html";
export { MapCanvas } from "./MapCanvas";
export type { LatLng, MapCanvasProps, MapColors, MapMarker, MapPadding, MarkerKind } from "./types";
