/**
 * useActiveTripRedirect — whenever the rider has a ride in progress (found at
 * launch, after a reconnect, or booked on another device) the live trip screen
 * takes over, wherever the rider is in the app. Screens that belong to the trip
 * itself (the trip screen, the cancel sheet, safety, the chat) are left alone.
 */
import { useSession } from "@platform/auth";
import { usePathname, useRouter } from "expo-router";
import { useEffect } from "react";
import { useActiveTrip } from "@/hooks/useTrips";

// The options screen hands over to the trip itself (after its slide-away animation).
const TRIP_SCREENS = ["/book/requested", "/book/options", "/cancel-reason", "/account/safety"];

export function useActiveTripRedirect() {
  const { isSignedIn } = useSession();
  const { data: active } = useActiveTrip();
  const pathname = usePathname();
  const router = useRouter();
  useEffect(() => {
    if (!isSignedIn || !active || TRIP_SCREENS.includes(pathname) || pathname.startsWith("/chat/")) return;
    router.replace({ pathname: "/book/requested", params: { tripId: active.id } });
  }, [isSignedIn, active, pathname, router]);
}
