"use client";
/** Trips hooks — the filtered list (URL-driven), one trip with its timeline, staff cancel. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";
import { useScope } from "@/lib/scope";

export interface TripFilter {
  status?: "active" | "scheduled" | "completed" | "cancelled";
  q?: string;
  offset?: number;
}

export const PAGE = 50;
const ACTIVE_POLL_MS = 10_000;

export function useTrips(f: TripFilter) {
  const { query } = useScope();
  return useQuery({
    queryKey: ["admin", "trips", f, query],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/trips", {
          params: {
            query: { ...query, status: f.status, q: f.q || undefined, limit: PAGE, offset: f.offset },
          },
        }),
      ),
    refetchInterval: f.status === "active" ? ACTIVE_POLL_MS : false,
    placeholderData: (prev) => prev,
  });
}

export function useTrip(id?: string) {
  return useQuery({
    queryKey: ["admin", "trip", id],
    queryFn: async () =>
      unwrap(await api.GET("/admin/trips/{id}", { params: { path: { id: id as string } } })),
    enabled: !!id,
  });
}

export function useCancelTrip() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, reason }: { id: string; reason: string }) =>
      unwrap(await api.POST("/admin/trips/{id}/cancel", { params: { path: { id } }, body: { reason } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin"] }),
  });
}

/** A completed trip's receipt (issued once, never changes). */
export function useTripReceipt(id: string, completed: boolean) {
  return useQuery({
    queryKey: ["admin", "receipt", id],
    queryFn: async () => unwrap(await api.GET("/admin/trips/{id}/receipt", { params: { path: { id } } })),
    enabled: completed,
    staleTime: Number.POSITIVE_INFINITY,
  });
}

/** Opens the receipt PDF in a new tab through a fresh 15-minute signed link. */
export function useOpenReceipt(id: string) {
  return useMutation({
    mutationFn: async (lang: "en" | "sw") => {
      const link = unwrap(
        await api.GET("/admin/trips/{id}/receipt/link", { params: { path: { id }, query: { lang } } }),
      );
      window.open(link.url, "_blank", "noopener");
    },
  });
}

/** Sets a trip's fare by hand before it is paid (audited). */
export function useSetPrice() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, amount, reason }: { id: string; amount: string; reason: string }) =>
      unwrap(
        await api.POST("/admin/trips/{id}/price", { params: { path: { id } }, body: { amount, reason } }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin"] }),
  });
}

/** What a completed trip's rider paid, got back and can still get back. */
export function useTripRefunds(id: string, completed: boolean) {
  return useQuery({
    queryKey: ["admin", "trip", id, "refunds"],
    queryFn: async () => unwrap(await api.GET("/admin/trips/{id}/refunds", { params: { path: { id } } })),
    enabled: completed,
  });
}

/** Refunds part or all of a completed trip (audited). */
export function useRefund() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      id,
      key,
      ...body
    }: {
      id: string;
      key: string; // Idempotency-Key: a retried request returns the same refund
      amount: string;
      borne_by: "PLATFORM" | "DRIVER";
      reason: string;
    }) =>
      unwrap(
        await api.POST("/admin/trips/{id}/refunds", {
          params: { path: { id }, header: { "Idempotency-Key": key } },
          body,
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin"] }),
  });
}
