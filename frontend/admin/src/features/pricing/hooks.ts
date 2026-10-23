"use client";
/**
 * Pricing hooks — the pricing setup of the selected country or region, and the
 * audited writes. Every write carries the scope from the side-menu picker, so
 * a price saved while a region is selected applies to that region only.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";
import { useScope } from "@/lib/scope";

const KEY = ["admin", "pricing"];

type ScopeQuery = { country?: string; area_id?: string };

export function usePricing() {
  const { query } = useScope();
  return useQuery({
    queryKey: [...KEY, query],
    queryFn: async () => unwrap(await api.GET("/admin/pricing", { params: { query } })),
  });
}

/** A write that runs with the current scope and refreshes every pricing view afterwards. */
function usePricingWrite<T>(fn: (v: T, scope: ScopeQuery) => Promise<unknown>) {
  const qc = useQueryClient();
  const { query } = useScope();
  return useMutation({
    mutationFn: (v: T) => fn(v, query),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}

export const useUpdateTariff = () =>
  usePricingWrite(async (v: { service: string; cls: string; body: Schemas["TariffInput"] }, scope) =>
    unwrap(
      await api.PUT("/admin/pricing/tariffs/{service}/{class}", {
        params: { path: { service: v.service, class: v.cls }, query: scope },
        body: v.body,
      }),
    ),
  );

/** Removes a region's own price so it uses the country price again. */
export const useDeleteTariffOverride = () =>
  usePricingWrite(async (v: { service: string; cls: string; reason: string }, scope) => {
    const { error, response } = await api.DELETE("/admin/pricing/tariffs/{service}/{class}", {
      params: {
        path: { service: v.service, class: v.cls },
        query: { country: scope.country, area_id: scope.area_id ?? "", reason: v.reason },
      },
    });
    if (error || !response.ok)
      throw new Error(
        (error as { error?: { message?: string } } | undefined)?.error?.message ??
          "Could not remove the region price",
      );
  });

export const useSaveRoute = () =>
  usePricingWrite(async (body: Schemas["FixedRouteInput"]) =>
    unwrap(await api.POST("/admin/pricing/fixed-routes", { body })),
  );

export const useDeleteRoute = () =>
  usePricingWrite(async (v: { id: string; reason: string }) =>
    unwrap(
      await api.DELETE("/admin/pricing/fixed-routes/{id}", {
        params: { path: { id: v.id }, query: { reason: v.reason } },
      }),
    ),
  );
