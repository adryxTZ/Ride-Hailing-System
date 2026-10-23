/**
 * useOnboarding — the driver's registration (cached; refreshed every minute
 * while waiting for review so an approval shows without a restart) and the
 * actions that change it. Every action returns the new checklist, which
 * replaces the cache.
 */
import { useSession } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type DriverProfileInput,
  fetchOnboarding,
  type Onboarding,
  saveProfile,
  saveVehicle,
  submitOnboarding,
  uploadDocument,
  type VehicleInput,
} from "@/services/onboarding";

export const ONBOARDING_KEY = ["driver", "onboarding"] as const;
const REVIEW_POLL_MS = 60_000;

export function useOnboarding() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ONBOARDING_KEY,
    queryFn: ({ signal }) => fetchOnboarding(signal),
    enabled: isSignedIn,
    refetchInterval: (q) => (q.state.data?.profile.status === "PENDING_REVIEW" ? REVIEW_POLL_MS : false),
  });
}

/** Shared shape: run an action, put the returned checklist in the cache. */
function useOnboardingAction<T>(fn: (v: T) => Promise<Onboarding>) {
  const qc = useQueryClient();
  return useMutation({ mutationFn: fn, onSuccess: (ob) => qc.setQueryData(ONBOARDING_KEY, ob) });
}

export const useSaveProfile = () => useOnboardingAction((v: DriverProfileInput) => saveProfile(v));
export const useSaveVehicle = () => useOnboardingAction((v: VehicleInput) => saveVehicle(v));
export const useSubmitOnboarding = () => useOnboardingAction((_: void) => submitOnboarding());
export const useUploadDocument = () =>
  useOnboardingAction(
    (v: { kind: string; uri: string; type: "image/jpeg" | "image/png"; expiresOn?: string }) =>
      uploadDocument(v.kind, v.uri, v.type, v.expiresOn),
  );
