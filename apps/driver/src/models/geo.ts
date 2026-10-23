/** Geometry helpers for booking screens: distances, route decoding and map framing. Pure functions. */
import type { Point } from "@/services/presence";

const EARTH_KM = 6371;
const POLYLINE_PRECISION = 1e5;

/** Great-circle distance in kilometres. */
export function distanceKm(a: Point, b: Point): number {
  const rad = (d: number) => (d * Math.PI) / 180;
  const dLat = rad(b.lat - a.lat);
  const dLng = rad(b.lng - a.lng);
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(rad(a.lat)) * Math.cos(rad(b.lat)) * Math.sin(dLng / 2) ** 2;
  return 2 * EARTH_KM * Math.asin(Math.sqrt(h));
}

/** Decodes a Google encoded polyline. Returns [] for empty/invalid input. */
export function decodePolyline(encoded?: string): Point[] {
  if (!encoded) return [];
  const out: Point[] = [];
  let i = 0;
  let lat = 0;
  let lng = 0;
  const next = () => {
    let result = 0;
    let shift = 0;
    let b: number;
    do {
      b = encoded.charCodeAt(i++) - 63;
      result |= (b & 0x1f) << shift;
      shift += 5;
    } while (b >= 0x20); // past the end charCodeAt is NaN, which ends the loop
    return result & 1 ? ~(result >> 1) : result >> 1;
  };
  while (i < encoded.length) {
    lat += next();
    lng += next();
    out.push({ lat: lat / POLYLINE_PRECISION, lng: lng / POLYLINE_PRECISION });
  }
  return out;
}

/** The line to draw between two stops: the routed polyline when we have one, else a straight segment. */
export function routeLine(pickup?: Point, dropoff?: Point, polyline?: string): Point[] {
  const decoded = decodePolyline(polyline);
  if (decoded.length > 1) return decoded;
  return pickup && dropoff ? [pickup, dropoff] : [];
}
