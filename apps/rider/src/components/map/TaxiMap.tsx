/**
 * TaxiMap — the full-bleed map behind every taxi screen. Data in, pins out:
 * nearby cars, the rider's own position, pickup ("Pickup here" / "Pickup · 2 min"),
 * drop-off (with arrival time) and the route line. It frames whatever it shows
 * inside `insets` so nothing hides behind floating bars or sheets.
 *
 * Drawn by @platform/maps (MapLibre + OpenFreeMap): Expo Go's built-in Google
 * Maps key is not authorised, so Google tiles cannot load there.
 */
import { MapCanvas, type MapMarker } from "@platform/maps";
import { useTheme } from "@platform/ui";
import { useMemo } from "react";
import { useCurrentLocation } from "@/hooks/useCurrentLocation";
import type { NearbyVehicle } from "@/services/supply";
import type { Point } from "@/services/trips";

const SELF_FALLBACK = "#4285F4";

export interface MapInsets {
  top: number;
  bottom: number;
}

interface Props {
  centre: Point;
  insets: MapInsets;
  cars?: NearbyVehicle[];
  pickup?: Point;
  pickupLabel?: string;
  /** Second line on the pickup pill, e.g. "2 min". */
  pickupValue?: string;
  dropoff?: Point;
  dropoffEta?: string;
  route?: Point[];
  /** Stops between pickup and drop-off, in order. */
  stops?: Point[];
  /** The assigned driver's car (live). */
  driver?: Point;
  /** Frame exactly these points (overrides the pickup/dropoff framing). */
  fitTo?: Point[];
  showsUser?: boolean;
  /** Fly to this zoom around `centre` (street level on "Ride requested"). */
  zoom?: number;
  /** Zoom the map opens at; a wide start plus `zoom` gives the fly-in. */
  initialZoom?: number;
  /** Bump to move the camera back to `centre`. */
  recenterKey?: number;
  onCentreChange?: (centre: Point) => void;
}

export function TaxiMap(p: Props) {
  const { palette } = useTheme();
  const { point: here } = useCurrentLocation();
  const showSelf = p.showsUser ?? true;

  const markers = useMemo<MapMarker[]>(() => {
    const out: MapMarker[] = (p.cars ?? []).map((c, i) => ({
      id: `car-${i}`,
      kind: "car",
      at: c.location,
      heading: c.heading,
    }));
    if (showSelf && here) out.push({ id: "self", kind: "self", at: here });
    if (p.driver) out.push({ id: "driver", kind: "car", at: p.driver });
    if (p.pickup)
      out.push({ id: "pickup", kind: "pickup", at: p.pickup, label: p.pickupLabel, value: p.pickupValue });
    (p.stops ?? []).forEach((st, i) => {
      out.push({ id: `stop-${i}`, kind: "stop", at: st });
    });
    if (p.dropoff)
      out.push({
        id: "dropoff",
        kind: "dropoff",
        at: p.dropoff,
        label: p.dropoffEta ? "Dropoff" : undefined,
        value: p.dropoffEta,
      });
    return out;
  }, [
    p.cars,
    showSelf,
    here,
    p.driver,
    p.pickup,
    p.pickupLabel,
    p.pickupValue,
    p.stops,
    p.dropoff,
    p.dropoffEta,
  ]);

  const fit = useMemo(
    () =>
      p.fitTo ?? (p.pickup && p.dropoff ? [...(p.route ?? []), ...(p.stops ?? []), p.pickup, p.dropoff] : []),
    [p.fitTo, p.pickup, p.dropoff, p.route, p.stops],
  );

  return (
    <MapCanvas
      centre={p.centre}
      padding={p.insets}
      colors={{
        primary: palette.primary,
        onPrimary: palette.on_primary ?? palette.background,
        text: palette.text,
        background: palette.background,
        self: palette.info ?? SELF_FALLBACK,
      }}
      markers={markers}
      route={p.route}
      fit={fit}
      zoom={p.zoom}
      initialZoom={p.initialZoom}
      recenterKey={p.recenterKey}
      onCentreChange={p.onCentreChange}
    />
  );
}
