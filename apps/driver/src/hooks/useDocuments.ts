/**
 * useDocuments — an approved driver's documents with expiry states, and
 * uploading a renewal (it waits for review beside the current one).
 */
import { useSession } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchDocuments, renewDocument } from "@/services/onboarding";

export const DOCUMENTS_KEY = ["driver", "documents"] as const;

export function useDocuments() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: DOCUMENTS_KEY,
    queryFn: ({ signal }) => fetchDocuments(signal),
    enabled: isSignedIn,
    staleTime: 30_000,
  });
}

export function useRenewDocument() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { kind: string; uri: string; type: "image/jpeg" | "image/png"; expiresOn?: string }) =>
      renewDocument(v.kind, v.uri, v.type, v.expiresOn),
    onSuccess: (docs) => {
      qc.setQueryData(DOCUMENTS_KEY, docs);
      void qc.invalidateQueries({ queryKey: ["driver", "wallet"] }); // holds show on the wallet
    },
  });
}
