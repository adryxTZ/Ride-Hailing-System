"use client";
/** Compliance hooks — each regulated country's switches and figures, the submissions outbox, and the audited actions. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";
import { useScope } from "@/lib/scope";

const KEY = ["admin", "compliance"];
const REFRESH_MS = 15_000; // the sweeper runs every minute

export function useRegulatorStatus() {
  return useQuery({
    queryKey: [...KEY, "status"],
    queryFn: async () => unwrap(await api.GET("/admin/compliance/regulator")).data,
    refetchInterval: REFRESH_MS,
  });
}

export interface SubmissionFilter {
  kind?: "DRIVER" | "VEHICLE" | "TRIP" | "DAILY_REPORT";
  status?: "PENDING" | "ACCEPTED" | "SKIPPED";
  stuck?: boolean;
  q?: string;
}

export function useSubmissions(f: SubmissionFilter) {
  const { scope } = useScope();
  return useQuery({
    queryKey: [...KEY, "submissions", f, scope.country],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/compliance/submissions", {
          params: {
            query: {
              country: scope.country || undefined,
              ...f,
              q: f.q || undefined,
              stuck: f.stuck || undefined,
            },
          },
        }),
      ).data,
    refetchInterval: REFRESH_MS,
    placeholderData: (prev) => prev,
  });
}

function useComplianceWrite<T>(fn: (v: T) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({ mutationFn: fn, onSuccess: () => qc.invalidateQueries({ queryKey: KEY }) });
}

export const useRegulatorSettings = () =>
  useComplianceWrite(
    async (v: { country: string; enabled?: boolean; fail_closed?: boolean; reason: string }) =>
      unwrap(
        await api.PATCH("/admin/compliance/regulator/{country}", {
          params: { path: { country: v.country } },
          body: { enabled: v.enabled, fail_closed: v.fail_closed, reason: v.reason },
        }),
      ),
  );

export const useRetrySubmission = () =>
  useComplianceWrite(async (v: { id: string; reason: string }) =>
    unwrap(
      await api.POST("/admin/compliance/submissions/{id}/retry", {
        params: { path: { id: v.id } },
        body: { reason: v.reason },
      }),
    ),
  );

export type DocumentType = Schemas["DocumentType"];

/** The documents the selected country asks drivers for. */
export function useDocumentTypes() {
  const { scope } = useScope();
  return useQuery({
    queryKey: [...KEY, "document-types", scope.country],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/compliance/document-types", {
          params: { query: { country: scope.country || undefined } },
        }),
      ).data,
    enabled: !!scope.country,
  });
}

export const useSaveDocumentType = () =>
  useComplianceWrite(async (v: DocumentType & { reason: string }) =>
    unwrap(await api.PUT("/admin/compliance/document-types", { body: v })),
  );

/** Approved drivers' documents of the selected country in one state. */
export function useAdminDocuments(state: "EXPIRING" | "EXPIRED" | "UNKNOWN" | "RENEWAL_PENDING" | "MISSING") {
  const { scope } = useScope();
  return useQuery({
    queryKey: [...KEY, "documents", state, scope.country],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/compliance/documents", {
          params: { query: { country: scope.country || undefined, state } },
        }),
      ).data,
    enabled: !!scope.country,
  });
}
