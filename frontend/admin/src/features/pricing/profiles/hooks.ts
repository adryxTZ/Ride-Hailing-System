"use client";
/** Profile hooks — the list, one profile, payees, and every staff change (each refreshes pricing data). */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";
import { unwrapGated } from "@/lib/approval";

const KEY = ["admin", "profiles"];

/** Where a profile can be applied: a region, or a district, zone or place (geofences). */
export type ScopeKind = "AREA" | "DISTRICT" | "ZONE" | "PLACE";

/** The kinds of third party a charge can be owed to. */
export const PAYEE_KINDS = [
  "MUNICIPALITY",
  "AIRPORT",
  "PARKING_OPERATOR",
  "TAX_AUTHORITY",
  "REGULATOR",
  "OTHER",
] as const;
export type PayeeKind = (typeof PAYEE_KINDS)[number];

export function useProfiles(country: string) {
  return useQuery({
    queryKey: [...KEY, "list", country],
    queryFn: async () =>
      unwrap(await api.GET("/admin/pricing/profiles", { params: { query: { country } } })).data,
    enabled: !!country,
  });
}

export function useProfile(id?: string) {
  return useQuery({
    queryKey: [...KEY, "one", id],
    queryFn: async () =>
      unwrap(await api.GET("/admin/pricing/profiles/{id}", { params: { path: { id: id as string } } })),
    enabled: !!id,
  });
}

export function usePayees(country: string) {
  return useQuery({
    queryKey: [...KEY, "payees", country],
    queryFn: async () =>
      unwrap(await api.GET("/admin/pricing/payees", { params: { query: { country } } })).data,
    enabled: !!country,
  });
}

function useWrite<T, R>(fn: (v: T) => Promise<R>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: KEY });
      void qc.invalidateQueries({ queryKey: ["admin", "pricing"] });
    },
  });
}

const path = (id: string) => ({ params: { path: { id } } });

export const useCreateProfile = () =>
  useWrite(async (b: { country: string; name: string; description?: string; reason: string }) =>
    unwrap(await api.POST("/admin/pricing/profiles", { body: b })),
  );

export const useUpdateProfile = () =>
  useWrite(
    async (a: {
      id: string;
      body: {
        name?: string;
        description?: string;
        active_from?: string;
        active_until?: string;
        clear_window?: boolean;
        reason: string;
      };
    }) => unwrap(await api.PATCH("/admin/pricing/profiles/{id}", { ...path(a.id), body: a.body })),
  );

export const usePublishVersion = () =>
  useWrite(
    async (a: {
      id: string;
      terms: Schemas["ProfileTerms"];
      charges: Schemas["ChargeRule"][];
      effective_from?: string;
      reason: string;
    }) =>
      unwrapGated(
        await api.POST("/admin/pricing/profiles/{id}/versions", {
          ...path(a.id),
          body: { terms: a.terms, charges: a.charges, effective_from: a.effective_from, reason: a.reason },
        }),
      ),
  );

export const useSetStatus = () =>
  useWrite(async (a: { id: string; status: "ACTIVE" | "INACTIVE"; reason: string }) =>
    unwrapGated(
      await api.POST("/admin/pricing/profiles/{id}/status", {
        ...path(a.id),
        body: { status: a.status, reason: a.reason },
      }),
    ),
  );

export const useAssign = () =>
  useWrite(
    async (a: { id: string; scope_kind: ScopeKind; scope_id: string; priority: number; reason: string }) =>
      unwrapGated(
        await api.POST("/admin/pricing/profiles/{id}/assignments", {
          ...path(a.id),
          body: { scope_kind: a.scope_kind, scope_id: a.scope_id, priority: a.priority, reason: a.reason },
        }),
      ),
  );

export const useUnassign = () =>
  useWrite(async (a: { id: string; assignmentId: string; reason: string }) =>
    unwrap(
      await api.DELETE("/admin/pricing/profiles/{id}/assignments/{assignmentId}", {
        params: { path: { id: a.id, assignmentId: a.assignmentId }, query: { reason: a.reason } },
      }),
    ),
  );

export const useMakeDefault = () =>
  useWrite(async (a: { id: string; reason: string }) =>
    unwrapGated(
      await api.POST("/admin/pricing/profiles/{id}/make-default", {
        ...path(a.id),
        body: { reason: a.reason },
      }),
    ),
  );

export const useCreatePayee = () =>
  useWrite(
    async (b: { country: string; name: string; kind: PayeeKind; reference?: string; reason: string }) =>
      unwrap(await api.POST("/admin/pricing/payees", { body: b })),
  );

export const useUpdatePayee = () =>
  useWrite(async (a: { id: string; status: "ACTIVE" | "INACTIVE"; reason: string }) =>
    unwrap(
      await api.PATCH("/admin/pricing/payees/{id}", {
        ...path(a.id),
        body: { status: a.status, reason: a.reason },
      }),
    ),
  );
