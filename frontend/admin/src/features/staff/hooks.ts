"use client";
/** Staff hooks — accounts, one person's page, roles, and the audited actions on them. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";
import { unwrapGated } from "@/lib/approval";

const KEY = ["admin", "staff"];
const ROLES = ["admin", "roles"];

export function useStaff() {
  return useQuery({ queryKey: KEY, queryFn: async () => unwrap(await api.GET("/admin/staff")) });
}

export function useStaffMember(id: string) {
  return useQuery({
    queryKey: [...KEY, id],
    queryFn: async () => unwrap(await api.GET("/admin/staff/{id}", { params: { path: { id } } })),
    refetchInterval: 30_000,
  });
}

export function useRoles() {
  return useQuery({ queryKey: ROLES, queryFn: async () => unwrap(await api.GET("/admin/roles")) });
}

/** Activity of one person: what they did (actor) or what was done to their account (target). */
export function usePersonAudit(id: string, kind: "actor" | "signins") {
  return useQuery({
    queryKey: ["admin", "audit", kind, id],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/audit", {
          params: {
            query:
              kind === "actor"
                ? { actor: id, limit: 100 }
                : { target: `user:${id}`, action: "staff.login", limit: 100 },
          },
        }),
      ).data,
  });
}

function useStaffWrite<T>(fn: (v: T) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: KEY });
      void qc.invalidateQueries({ queryKey: ROLES });
      void qc.invalidateQueries({ queryKey: ["admin", "audit"] });
    },
  });
}

/** Throws the server's message for calls that answer 204. */
function check(r: { error?: unknown; response: Response }) {
  if (r.error || !r.response.ok) {
    throw new Error(
      (r.error as { error?: { message?: string } } | undefined)?.error?.message ?? "The change was not saved",
    );
  }
}

export function useSetStaff() {
  return useStaffWrite(async (body: Schemas["StaffInput"]) =>
    unwrapGated(await api.PUT("/admin/staff", { body })),
  );
}

export const useEndSessions = () =>
  useStaffWrite(async (v: { id: string; sessionId?: string; reason: string }) =>
    check(
      await api.POST("/admin/staff/{id}/sessions/end", {
        params: { path: { id: v.id } },
        body: { session_id: v.sessionId, reason: v.reason },
      }),
    ),
  );

export const useSetActive = () =>
  useStaffWrite(async (v: { id: string; active: boolean; reason: string }) =>
    check(
      v.active
        ? await api.POST("/admin/staff/{id}/reactivate", {
            params: { path: { id: v.id } },
            body: { reason: v.reason },
          })
        : await api.POST("/admin/staff/{id}/deactivate", {
            params: { path: { id: v.id } },
            body: { reason: v.reason },
          }),
    ),
  );

export const useSetAccess = () =>
  useStaffWrite(async (v: { id: string; access: Schemas["StaffAccess"][]; reason: string }) =>
    check(
      await api.PUT("/admin/staff/{id}/access", {
        params: { path: { id: v.id } },
        body: { access: v.access, reason: v.reason },
      }),
    ),
  );

export const useSaveRole = () =>
  useStaffWrite(async (v: { id?: string; body: Schemas["RoleInput"] }) =>
    v.id
      ? unwrap(await api.PUT("/admin/roles/{id}", { params: { path: { id: v.id } }, body: v.body }))
      : unwrap(await api.POST("/admin/roles", { body: v.body })),
  );

export const useDeleteRole = () =>
  useStaffWrite(async (v: { id: string; reason: string }) =>
    check(
      await api.DELETE("/admin/roles/{id}", { params: { path: { id: v.id }, query: { reason: v.reason } } }),
    ),
  );
