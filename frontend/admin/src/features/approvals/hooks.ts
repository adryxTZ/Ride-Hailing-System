"use client";
/**
 * Approvals hooks — the inbox (pending, mine, done), one request and the
 * decisions on it, the nav badge count, and the workflow editor's data.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";

export type ApprovalRequest = Schemas["ApprovalRequest"];
export type Definition = Schemas["ApprovalDefinition"];
export type Step = Schemas["ApprovalStep"];
export type ActionWorkflow = Schemas["ApprovalActionWorkflow"];
export type Box = "pending" | "mine" | "done";

const KEY = ["admin", "approvals"];

/** How often the badge and an open inbox look for new requests. */
const REFRESH_MS = 30_000;

/** The server caps a page at 200. */
const LIST_LIMIT = 100;

/** Pending requests the signed-in person could decide now (the nav badge). */
export function useApprovalCount() {
  return useQuery({
    queryKey: [...KEY, "summary"],
    queryFn: async () => unwrap(await api.GET("/admin/approvals/requests/summary")).actionable,
    refetchInterval: REFRESH_MS,
  });
}

/** One inbox box. */
export function useApprovalRequests(box: Box) {
  return useQuery({
    queryKey: [...KEY, "list", box],
    queryFn: async () =>
      unwrap(await api.GET("/admin/approvals/requests", { params: { query: { box, limit: LIST_LIMIT } } }))
        .data,
    refetchInterval: REFRESH_MS,
  });
}

export function useApprovalRequest(id: string) {
  return useQuery({
    queryKey: [...KEY, "one", id],
    queryFn: async () =>
      unwrap(await api.GET("/admin/approvals/requests/{id}", { params: { path: { id } } })),
  });
}

/** Approve, reject or (the requester) cancel, with a note; everything that may have changed refreshes. */
export function useDecideApproval(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ decision, note }: { decision: "approve" | "reject" | "cancel"; note: string }) => {
      const opts = { params: { path: { id } }, body: { note } };
      if (decision === "approve")
        return unwrap(await api.POST("/admin/approvals/requests/{id}/approve", opts));
      if (decision === "reject") return unwrap(await api.POST("/admin/approvals/requests/{id}/reject", opts));
      return unwrap(await api.POST("/admin/approvals/requests/{id}/cancel", opts));
    },
    onSettled: () => qc.invalidateQueries({ queryKey: ["admin"] }),
  });
}

/** Every approvable action with its workflow, and the roles and permissions steps may name. */
export function useWorkflows() {
  return useQuery({
    queryKey: [...KEY, "workflows"],
    queryFn: async () => unwrap(await api.GET("/admin/approvals/workflows")),
  });
}

/** One action's workflow and its saved versions. */
export function useWorkflow(action: string) {
  return useQuery({
    queryKey: [...KEY, "workflows", action],
    queryFn: async () =>
      unwrap(await api.GET("/admin/approvals/workflows/{action}", { params: { path: { action } } })),
  });
}

export function useSaveWorkflow(action: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: Schemas["ApprovalWorkflowInput"]) =>
      unwrap(await api.PUT("/admin/approvals/workflows/{action}", { params: { path: { action } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: [...KEY, "workflows"] }),
  });
}
