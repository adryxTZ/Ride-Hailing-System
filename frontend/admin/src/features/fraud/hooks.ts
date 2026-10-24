"use client";
/** Fraud hooks — the review queue, one flag, and the review (dismiss or confirm). */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";

export type Flag = Schemas["FraudFlag"];
export type FlagStatus = Flag["status"];
export type Rule = Flag["rule"];

const KEY = ["admin", "fraud"];

/** The queue for a country, filtered by status and rule (newest first). */
export function useFlags(country: string, status: FlagStatus | "", rule: Rule | "", subjectId?: string) {
  return useQuery({
    queryKey: [...KEY, "list", country, status, rule, subjectId],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/fraud/flags", {
          params: {
            query: {
              country: country || undefined,
              status: status || undefined,
              rule: rule || undefined,
              subject_id: subjectId,
              limit: 100,
            },
          },
        }),
      ).data,
    enabled: !!country || !!subjectId,
  });
}

export function useFlag(id: string) {
  return useQuery({
    queryKey: [...KEY, "one", id],
    queryFn: async () => unwrap(await api.GET("/admin/fraud/flags/{id}", { params: { path: { id } } })),
  });
}

export function useReview(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: { decision: "DISMISS" | "CONFIRM"; note: string }) =>
      unwrap(await api.POST("/admin/fraud/flags/{id}/review", { params: { path: { id } }, body })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: KEY });
      void qc.invalidateQueries({ queryKey: ["admin", "overview"] });
    },
  });
}
