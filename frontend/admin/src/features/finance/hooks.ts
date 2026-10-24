"use client";
/**
 * Finance hooks — ledger totals with driver balances, the journal, and the hash-chain check.
 * Everything is for the country picked in the top bar, in its own currency.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";
import { unwrapGated } from "@/lib/approval";
import { useScope } from "@/lib/scope";

const PAYINS_LIMIT = 20; // the latest ones; older payments are in the journal
const PAYINS_REFRESH_MS = 5_000; // a prompt is answered within seconds

const JOURNAL_LIMIT = 50;

/** The selected country only; finance is per country (currency), never per region. */
function useCountry() {
  const { scope } = useScope();
  return { country: scope.country || undefined };
}

export function useFinance() {
  const query = useCountry();
  return useQuery({
    queryKey: ["admin", "finance", query],
    queryFn: async () => unwrap(await api.GET("/admin/finance", { params: { query } })),
  });
}

export function useJournal() {
  const query = useCountry();
  return useQuery({
    queryKey: ["admin", "finance", "journal", query],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/finance/journal", { params: { query: { ...query, limit: JOURNAL_LIMIT } } }),
      ),
  });
}

export function useVerifyLedger() {
  return useMutation({ mutationFn: async () => unwrap(await api.POST("/admin/finance/verify")) });
}

export function useHolds() {
  const query = useCountry();
  return useQuery({
    queryKey: ["admin", "finance", "holds", query],
    queryFn: async () => unwrap(await api.GET("/admin/finance/holds", { params: { query } })),
  });
}

export function usePayins(status: string, purpose = "") {
  const country = useCountry();
  return useQuery({
    queryKey: ["admin", "finance", "payins", status, purpose, country],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/finance/payins", {
          params: {
            query: {
              ...country,
              purpose: (purpose || undefined) as "DRIVER_DEBT" | "RIDE_FARE" | undefined,
              status: (status || undefined) as "PENDING" | "COLLECTED" | "FAILED" | undefined,
              limit: PAYINS_LIMIT,
            },
          },
        }),
      ),
    // A prompt is answered within seconds; keep the list fresh while any payment is open.
    refetchInterval: (q) =>
      q.state.data?.data.some((p) => p.status === "PENDING") ? PAYINS_REFRESH_MS : false,
  });
}

/** Place or release a staff hold; every finance view refreshes afterwards. */
export function useHoldAction() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { driverId: string; release: boolean; reason: string }) => {
      const opts = { params: { path: { id: v.driverId } }, body: { reason: v.reason } };
      return v.release
        ? unwrap(await api.POST("/admin/drivers/{id}/hold/release", opts))
        : unwrap(await api.POST("/admin/drivers/{id}/hold", opts));
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "finance"] }),
  });
}

const PAYOUTS_REFRESH_MS = 10_000; // payouts move in seconds once approved

/** Driver cash-outs in the selected country's currency, open ones first. */
export function usePayouts(status: string) {
  const country = useCountry();
  return useQuery({
    queryKey: ["admin", "finance", "payouts", status, country],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/finance/payouts", {
          params: { query: { ...country, status: (status || undefined) as "REQUESTED" | undefined } },
        }),
      ),
    refetchInterval: (q) =>
      q.state.data?.data.some((p) => p.status === "PENDING") ? PAYOUTS_REFRESH_MS : false,
  });
}

/** Approve or reject a requested cash-out with a reason; every finance view refreshes afterwards. */
export function usePayoutDecision() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { id: string; approve: boolean; reason: string }) => {
      const opts = { params: { path: { id: v.id } }, body: { reason: v.reason } };
      return v.approve
        ? unwrapGated(await api.POST("/admin/finance/payouts/{id}/approve", opts))
        : unwrap(await api.POST("/admin/finance/payouts/{id}/reject", opts));
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "finance"] }),
  });
}
