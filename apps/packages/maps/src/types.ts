/** Map data model shared by every screen: plain data in, the canvas draws it. */

export interface LatLng {
  lat: number;
  lng: number;
}

/** What a pin looks like. Labels are already localised by the caller. */
export type MarkerKind = "pickup" | "dropoff" | "stop" | "car" | "self";

export interface MapMarker {
  id: string;
  at: LatLng;
  kind: MarkerKind;
  /** First line of a pin's label, e.g. "Pickup here", "Dropoff". */
  label?: string;
  /** Bold second line, e.g. "2 min", "3:50 pm". */
  value?: string;
  /** Degrees clockwise from north (cars). */
  heading?: number;
}

/** Brand colours the page needs; taken from the server theme by the caller. */
export interface MapColors {
  primary: string;
  onPrimary: string;
  text: string;
  background: string;
  self: string;
}

/** Screen space covered by floating bars and sheets; the camera centres in what is left. */
export interface MapPadding {
  top: number;
  bottom: number;
  left?: number;
  right?: number;
}

export interface MapCanvasProps {
  centre: LatLng;
  padding: MapPadding;
  colors: MapColors;
  markers?: MapMarker[];
  route?: LatLng[];
  /** Frame these points (both stops and the route); wins over `centre`. */
  fit?: LatLng[];
  /** Fly to this zoom around `centre` (e.g. 16.5 on "Ride requested"). */
  zoom?: number;
  /** Zoom the map opens at before any fly-in. */
  initialZoom?: number;
  /** Bump to move the camera back to `centre`. */
  recenterKey?: number;
  /** Vector style URL; defaults to OpenFreeMap Positron (free, no key). */
  styleUrl?: string;
  /** Fires when the user (not the app) finished moving the map. */
  onCentreChange?: (centre: LatLng) => void;
}

/** The state pushed into the page on every change. */
export interface MapState {
  centre: LatLng;
  padding: Required<MapPadding>;
  colors: MapColors;
  markers: MapMarker[];
  route: LatLng[];
  fit: LatLng[];
  zoom?: number;
  recenterKey: number;
}
