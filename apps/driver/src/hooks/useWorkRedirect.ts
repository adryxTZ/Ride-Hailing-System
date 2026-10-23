/**
 * useWorkRedirect — a new ride offer or an assigned trip always brings the driver
 * to Home, where the offer card / trip panel lives, from any tab or screen.
 */
import { usePathname, useRouter } from "expo-router";
import { useEffect } from "react";
import { useDriverTrip, useOffer } from "@/hooks/useDriverTrips";

export function useWorkRedirect() {
  const { data: offer } = useOffer();
  const { data: trip } = useDriverTrip();
  const pathname = usePathname();
  const router = useRouter();
  const busy = !!offer || !!trip;
  useEffect(() => {
    // The trip's chat belongs to the work; everything else yields to it.
    if (busy && pathname !== "/" && !pathname.startsWith("/chat/")) router.navigate("/");
  }, [busy, pathname, router]);
}
