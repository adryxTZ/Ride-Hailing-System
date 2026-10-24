/**
 * Consent hooks — the Privacy Notice gate's behaviour:
 *  - useConsentWatch: keeps the consent flag in step with GET /me (start + foreground);
 *  - useConsentDocument: the current notice for this app;
 *  - useAcceptConsent: accepts it; a stale version refetches the notice and says so.
 */
import { ApiError, CONSENT_VERSION_STALE, useConsentSync } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useProfile } from "@/hooks/useProfile";
import { consent } from "@/services/api";

const DOCUMENT_KEY = ["consent-document"] as const;
const DOCUMENT_STALE_MS = 5 * 60_000;

export function useConsentWatch() {
  const { data: me, refetch } = useProfile();
  useConsentSync(me?.consent?.required, refetch);
}

export function useConsentDocument() {
  return useQuery({
    queryKey: DOCUMENT_KEY,
    queryFn: ({ signal }) => consent.document(signal),
    staleTime: DOCUMENT_STALE_MS,
  });
}

export function useAcceptConsent() {
  const qc = useQueryClient();
  // Set when the notice changed while the screen was open: the new one is shown as an update.
  const [stale, setStale] = useState(false);
  const mutation = useMutation({
    mutationFn: (version: number) => consent.accept(version),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["me"] }),
    onError: (e) => {
      if (!(e instanceof ApiError) || e.code !== CONSENT_VERSION_STALE) return;
      setStale(true);
      void qc.invalidateQueries({ queryKey: DOCUMENT_KEY });
    },
  });
  return { ...mutation, stale };
}
