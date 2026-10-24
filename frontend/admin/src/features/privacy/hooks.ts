"use client";
/**
 * Privacy hooks — the data-protection queue (download my data, delete my
 * account), one request, approve / reject with a reason, and the retention
 * rules' last daily run.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";

export type PrivacyRequest = Schemas["PrivacyRequest"];
export type RequestKind = PrivacyRequest["kind"];
export type RequestStatus = PrivacyRequest["status"];
export type Blocker = PrivacyRequest["blockers"][number];

const KEY = ["admin", "privacy"];

/** The server caps a page at 200; the queue is small, so one page covers it. */
const LIST_LIMIT = 200;

/** Statuses that still need staff (an export being built is still open). */
export const OPEN_STATUSES: readonly RequestStatus[] = ["REQUESTED", "IN_PROGRESS"];

/** Every request of a kind (open ones first, by due date); screens group them by status. */
export function useRequests(kind: RequestKind | "") {
  return useQuery({
    queryKey: [...KEY, "list", kind],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/privacy/requests", {
          params: { query: { kind: kind || undefined, limit: LIST_LIMIT } },
        }),
      ).data,
  });
}

/** One request; an open deletion carries what blocks it right now. */
export function useRequest(id: string) {
  return useQuery({
    queryKey: [...KEY, "one", id],
    queryFn: async () => unwrap(await api.GET("/admin/privacy/requests/{id}", { params: { path: { id } } })),
  });
}

/** Approve or reject with a reason; refreshes the queue, the request and Overview. */
export function useDecide(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ decision, reason }: { decision: "approve" | "reject"; reason: string }) => {
      const opts = { params: { path: { id } }, body: { reason } };
      return decision === "approve"
        ? unwrap(await api.POST("/admin/privacy/requests/{id}/approve", opts))
        : unwrap(await api.POST("/admin/privacy/requests/{id}/reject", opts));
    },
    // Settled, not success: a DELETE_BLOCKED answer refreshes the blockers shown.
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: KEY });
      void qc.invalidateQueries({ queryKey: ["admin", "overview"] });
    },
  });
}

/** Retention rules with their period and last daily run. */
export function useRetention() {
  return useQuery({
    queryKey: [...KEY, "retention"],
    queryFn: async () => unwrap(await api.GET("/admin/privacy/retention")).data,
  });
}
