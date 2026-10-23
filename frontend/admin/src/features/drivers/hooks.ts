"use client";
/** Drivers hooks — list with status counts, one driver's application, and the review decisions. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";
import { useScope } from "@/lib/scope";

export const DRIVER_PAGE = 50;
type Status = "ONBOARDING" | "PENDING_REVIEW" | "APPROVED" | "REJECTED" | "SUSPENDED";

/** Drivers of the country (and region) picked in the top bar. */
export function useDrivers(status?: Status, q?: string, offset = 0) {
  const { query } = useScope();
  return useQuery({
    queryKey: ["admin", "drivers", status, q, offset, query],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/drivers", {
          params: { query: { ...query, status, q: q || undefined, limit: DRIVER_PAGE, offset } },
        }),
      ),
    placeholderData: (prev) => prev,
  });
}

export function useDriver(id?: string) {
  return useQuery({
    queryKey: ["admin", "driver", id],
    queryFn: async () =>
      unwrap(await api.GET("/admin/drivers/{id}", { params: { path: { id: id as string } } })),
    enabled: !!id,
  });
}

/** Approve / reject / suspend / reinstate, and per-document review; every call refreshes the lists. */
export function useDriverDecision() {
  const qc = useQueryClient();
  const done = () => qc.invalidateQueries({ queryKey: ["admin"] });
  return useMutation({
    mutationFn: async (a: {
      id: string;
      action: "approve" | "reject" | "suspend" | "reinstate";
      reason?: string;
    }) => {
      const path = { params: { path: { id: a.id } } };
      switch (a.action) {
        case "approve":
          return unwrap(await api.POST("/admin/drivers/{id}/approve", path));
        case "reject":
          return unwrap(
            await api.POST("/admin/drivers/{id}/reject", { ...path, body: { reason: a.reason ?? "" } }),
          );
        case "suspend":
          return unwrap(
            await api.POST("/admin/drivers/{id}/suspend", { ...path, body: { reason: a.reason ?? "" } }),
          );
        default:
          return unwrap(
            await api.POST("/admin/drivers/{id}/reinstate", { ...path, body: { reason: a.reason ?? "" } }),
          );
      }
    },
    onSuccess: done,
  });
}

export function useDocumentReview() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (a: { id: string; docId: string; approve: boolean; reason?: string }) =>
      unwrap(
        await api.POST("/admin/drivers/{id}/documents/{docId}/review", {
          params: { path: { id: a.id, docId: a.docId } },
          body: { approve: a.approve, reason: a.reason },
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin"] }),
  });
}
