/** useCurrentLocation — asks for foreground permission once and returns the device position. */
import * as Location from "expo-location";
import { useEffect, useState } from "react";
import type { Point } from "@/services/presence";

// Dar es Salaam centre: the map has somewhere sensible to open before a fix arrives.
export const DEFAULT_CENTRE: Point = { lat: -6.7924, lng: 39.2083 };

export function useCurrentLocation() {
  const [point, setPoint] = useState<Point>();
  const [denied, setDenied] = useState(false);
  const [mocked, setMocked] = useState(false); // Android's mock-location flag (sent when going online)
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const perm = await Location.requestForegroundPermissionsAsync();
      if (perm.status !== "granted") {
        if (!cancelled) setDenied(true);
        return;
      }
      const pos = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.Balanced });
      if (cancelled) return;
      setPoint({ lat: pos.coords.latitude, lng: pos.coords.longitude });
      setMocked(pos.mocked === true);
    })().catch(() => setDenied(true));
    return () => {
      cancelled = true;
    };
  }, []);
  return { point, denied, mocked };
}
