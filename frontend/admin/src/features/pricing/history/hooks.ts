"use client";
/** Pricing history hooks — "as of", the assignment timeline, the simulator and the records CSV. */
import { useMutation, useQuery } from "@tanstack/react-query";
import { ApiError, api, type Schemas, unwrap } from "@/lib/api";

export type LatLng = { lat: number; lng: number };
export type AsOf = Schemas["PricingAsOf"];
export type Timeline = Schemas["PricingTimeline"];
export type Simulation = Schemas["PricingSimulation"];
export type SimulateInput = Schemas["SimulateInput"];

/** What priced a point (and an optional drop-off) at a past time; runs when a point and a time are set. */
export function useAsOf(country: string, at: string, pickup?: LatLng, dropoff?: LatLng) {
  return useQuery({
    queryKey: ["admin", "pricing", "as-of", country, at, pickup, dropoff],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/pricing/as-of", {
          params: {
            query: {
              country,
              at,
              lat: pickup?.lat ?? 0,
              lng: pickup?.lng ?? 0,
              dropoff_lat: dropoff?.lat,
              dropoff_lng: dropoff?.lng,
            },
          },
        }),
      ),
    enabled: !!country && !!at && !!pickup,
  });
}

/** Assignments and profile events overlapping [from, to]. */
export function useTimeline(country: string, from: string, to: string, scopeKind?: string) {
  return useQuery({
    queryKey: ["admin", "pricing", "history", country, from, to, scopeKind],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/pricing/history", {
          params: {
            query: {
              country,
              from,
              to,
              scope_kind: (scopeKind || undefined) as "AREA" | undefined,
            },
          },
        }),
      ),
    enabled: !!country && !!from && !!to,
  });
}

/** Prices a trip without saving it. */
export function useSimulate() {
  return useMutation({
    mutationFn: async (body: SimulateInput) => unwrap(await api.POST("/admin/pricing/simulate", { body })),
  });
}

/** Downloads finalised pricing records as a CSV file. */
export function useExportRecords() {
  return useMutation({
    mutationFn: async ({ country, from, to }: { country: string; from: string; to: string }) => {
      const r = await api.GET("/admin/pricing/records.csv", {
        params: { query: { country, from, to } },
        parseAs: "blob",
      });
      if (r.error || !r.data) {
        const e = (r.error as { error?: { message?: string; code?: string } } | undefined)?.error;
        throw new ApiError(e?.message ?? "Export failed", e?.code, r.response.status);
      }
      const url = URL.createObjectURL(r.data as Blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `pricing-records-${country}-${from.slice(0, 10)}-${to.slice(0, 10)}.csv`;
      link.click();
      URL.revokeObjectURL(url);
    },
  });
}

/** An ISO time from a datetime-local input value (local time). */
export const isoOf = (local: string) => (local ? new Date(local).toISOString() : "");

/** A datetime-local value for a Date (local time, minutes). */
export function localOf(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
