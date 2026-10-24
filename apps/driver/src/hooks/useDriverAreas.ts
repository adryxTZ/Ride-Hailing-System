/** useDriverAreas — the regions offered in the sign-up form (cached for a few minutes). */
import { useSession } from "@platform/auth";
import { useQuery } from "@tanstack/react-query";
import { fetchDriverAreas } from "@/services/onboarding";

const AREAS_STALE_MS = 5 * 60_000;

export function useDriverAreas() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["driver", "areas"],
    queryFn: ({ signal }) => fetchDriverAreas(signal),
    enabled: isSignedIn,
    staleTime: AREAS_STALE_MS,
  });
}
