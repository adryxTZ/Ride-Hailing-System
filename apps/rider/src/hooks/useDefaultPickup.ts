/**
 * useDefaultPickup — fills the booking pickup with the device position the first
 * time it is known, named after a saved place within reach ("Home") or else the
 * street / landmark the rider stands on (reverse lookup), never a bare
 * "Current location" once a name is known.
 */
import { useEffect } from "react";
import { useCurrentLocation } from "@/hooks/useCurrentLocation";
import { usePlaces } from "@/hooks/useProfile";
import { distanceKm } from "@/models/geo";
import { useBookingStore } from "@/services/bookingStore";
import { reversePlace } from "@/services/places";

const SAVED_PLACE_RADIUS_KM = 0.15;
export const CURRENT_LOCATION = "Current location";

export function useDefaultPickup() {
  const { point } = useCurrentLocation();
  const { data: places } = usePlaces();
  const pickup = useBookingStore((s) => s.pickup);
  const setPickup = useBookingStore((s) => s.setPickup);

  useEffect(() => {
    if (!point || pickup) return;
    const near = places?.find((p) => distanceKm(point, { lat: p.lat, lng: p.lng }) <= SAVED_PLACE_RADIUS_KM);
    setPickup({ point, address: near?.label ?? CURRENT_LOCATION, source: near ? "saved" : "gps" });
  }, [point, pickup, places, setPickup]);

  // Name the GPS pickup once (street or landmark) if it is still the generic label.
  useEffect(() => {
    if (!pickup || pickup.address !== CURRENT_LOCATION) return;
    let cancelled = false;
    reversePlace(pickup.point).then((place) => {
      const current = useBookingStore.getState().pickup;
      if (
        !cancelled &&
        place?.name &&
        current?.point === pickup.point &&
        current.address === CURRENT_LOCATION
      ) {
        setPickup({ point: pickup.point, address: place.name, source: "gps" });
      }
    });
    return () => {
      cancelled = true;
    };
  }, [pickup, setPickup]);
  return point;
}
