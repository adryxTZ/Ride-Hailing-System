/** Geometry helpers for the geofence map: GeoJSON features from geofences, circles for places, rings from clicks. */
import type { Schemas } from "@/lib/api";

type Geofence = Schemas["Geofence"];
type Position = [number, number];

/** Points on a place's circle (the map has no circle geometry). */
const CIRCLE_STEPS = 64;
const METRES_PER_DEG_LAT = 111_320;

/** A circle of radius metres around a centre, as a closed lng/lat ring. */
export function circleRing(lat: number, lng: number, radiusM: number): Position[] {
  const ring: Position[] = [];
  for (let i = 0; i <= CIRCLE_STEPS; i++) {
    const a = (i / CIRCLE_STEPS) * 2 * Math.PI;
    const dLat = (radiusM * Math.sin(a)) / METRES_PER_DEG_LAT;
    const dLng = (radiusM * Math.cos(a)) / (METRES_PER_DEG_LAT * Math.cos((lat * Math.PI) / 180));
    ring.push([lng + dLng, lat + dLat]);
  }
  return ring;
}

/** A geofence as a GeoJSON feature (places become their circle). */
export function featureOf(f: Geofence, selected = false): GeoJSON.Feature | null {
  const props = { id: f.id, kind: f.kind, name: f.name, selected, inactive: f.status !== "ACTIVE" };
  const c = f.center;
  if (f.kind === "PLACE" && c?.lat !== undefined && c.lng !== undefined && f.radius_m) {
    return {
      type: "Feature",
      properties: props,
      geometry: { type: "Polygon", coordinates: [circleRing(c.lat, c.lng, f.radius_m)] },
    };
  }
  if (f.geometry?.coordinates) {
    return {
      type: "Feature",
      properties: props,
      geometry: { type: "MultiPolygon", coordinates: f.geometry.coordinates as Position[][][] },
    };
  }
  return null;
}

/** A closed polygon from clicked points (at least 3), or null. */
export function polygonFrom(points: Position[]): Schemas["GeofenceShape"]["geometry"] | null {
  if (points.length < 3) return null;
  return { type: "Polygon", coordinates: [[...points, points[0]]] };
}

/** The bounding box of every shape, for framing the map. */
export function boundsOf(fences: Geofence[]): [number, number, number, number] | null {
  const b = fences.reduce<[number, number, number, number] | null>((acc, f) => {
    const [w, s, e, n] = f.bbox;
    if (!acc) return [w, s, e, n];
    return [Math.min(acc[0], w), Math.min(acc[1], s), Math.max(acc[2], e), Math.max(acc[3], n)];
  }, null);
  return b;
}
