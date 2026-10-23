/**
 * Presence hooks — the driver's online state and the GPS stream that keeps it fresh.
 *
 * While online the app sends its position on a fixed heartbeat (every
 * PING_INTERVAL_MS, faster on a trip) whether or not the phone moved — the server
 * treats silence beyond its staleness window as offline. GPS updates between beats
 * only refresh the position that the next beat sends. The online status itself
 * is re-read from the server so the screen never claims "Online" when riders
 * cannot see the driver.
 */
import { useSession } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Location from "expo-location";
import { useEffect, useRef } from "react";
import {
  type DriverStatus,
  type DriverStatusInput,
  fetchStatus,
  postLocation,
  setStatus,
} from "@/services/presence";

const STATUS_KEY = ["driver", "status"] as const;
const PING_INTERVAL_MS = 10_000;
const TRIP_PING_INTERVAL_MS = 4_000; // the rider watches the car move during a trip
const STATUS_RECHECK_MS = 30_000;

export function useDriverStatus() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: STATUS_KEY,
    queryFn: ({ signal }) => fetchStatus(signal),
    enabled: isSignedIn,
    refetchInterval: STATUS_RECHECK_MS,
  });
}

export function useSetStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: DriverStatusInput) => setStatus(body),
    onSuccess: (status) => qc.setQueryData<DriverStatus>(STATUS_KEY, status),
  });
}

/** Sends the driver's position on a heartbeat while `online` (faster while on a trip). */
export function useLocationStream(online: boolean, onTrip = false) {
  const interval = onTrip ? TRIP_PING_INTERVAL_MS : PING_INTERVAL_MS;
  const qc = useQueryClient();
  const last = useRef<Location.LocationObject | undefined>(undefined);
  useEffect(() => {
    if (!online) return;
    let sub: Location.LocationSubscription | undefined;
    let cancelled = false;
    const beat = async () => {
      const pos = last.current ?? (await Location.getLastKnownPositionAsync().catch(() => null)) ?? undefined;
      if (!pos || cancelled) return;
      postLocation({
        location: { lat: pos.coords.latitude, lng: pos.coords.longitude },
        heading: pos.coords.heading ?? 0,
        speed_mps: pos.coords.speed ?? 0,
        client_at: new Date(pos.timestamp).toISOString(),
      })
        .then((status) => qc.setQueryData<DriverStatus>(STATUS_KEY, status))
        .catch(() => qc.invalidateQueries({ queryKey: STATUS_KEY })); // e.g. DRIVER_NOT_ONLINE → resync
    };
    const timer = setInterval(() => void beat(), interval);
    (async () => {
      const perm = await Location.requestForegroundPermissionsAsync();
      if (perm.status !== "granted" || cancelled) return;
      sub = await Location.watchPositionAsync(
        { accuracy: Location.Accuracy.High, timeInterval: interval, distanceInterval: 0 },
        (pos) => {
          last.current = pos; // the next beat sends the newest fix
        },
      );
      void beat();
    })();
    return () => {
      cancelled = true;
      clearInterval(timer);
      sub?.remove();
    };
  }, [online, interval, qc]);
}
