/**
 * Privacy hooks — my data-protection requests (React Query over the privacy
 * service): the list, asking for an export or a deletion, cancelling, and
 * opening a ready export's download link.
 */
import { useSession } from "@platform/auth";
import type { PrivacyKind } from "@platform/privacy";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Linking } from "react-native";
import { privacy } from "@/services/privacy";

const REQUESTS_KEY = ["privacy-requests"] as const;
const REQUESTS_STALE_MS = 30_000;

export function usePrivacyRequests() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: REQUESTS_KEY,
    queryFn: ({ signal }) => privacy.list(signal),
    staleTime: REQUESTS_STALE_MS,
    enabled: isSignedIn,
  });
}

/** Mutations that change the list refresh it, whether they succeed or not (409s mean it moved). */
function useRefreshingMutation<T>(fn: (arg: T) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSettled: () => void qc.invalidateQueries({ queryKey: REQUESTS_KEY }),
  });
}

export function useCreatePrivacyRequest() {
  return useRefreshingMutation((kind: PrivacyKind) => privacy.create(kind));
}

export function useCancelPrivacyRequest() {
  return useRefreshingMutation((id: string) => privacy.cancel(id));
}

/** Fetches a fresh link for a ready export and opens it in the browser. */
export function useDownloadExport() {
  return useRefreshingMutation(async (id: string) => {
    const link = await privacy.downloadLink(id);
    await Linking.openURL(link.url);
  });
}
