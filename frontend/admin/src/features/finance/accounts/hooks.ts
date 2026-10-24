"use client";
/**
 * Finance accounts hooks — the accounts list, one account's entries, the CSV
 * export, owed rider refunds and recording a payment. Everything is for the
 * country picked in the top bar (its currency) and the period in the URL.
 */
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { ApiError, api, unwrap } from "@/lib/api";
import { useScope } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import type { Method } from "./labels";

/** Entries per page on an account's page. */
export const ENTRIES_PAGE = 50;

/** A period: first and last day (inclusive), YYYY-MM-DD in local time. */
export interface Period {
  from: string;
  to: string;
}

const pad = (n: number) => String(n).padStart(2, "0");
/** YYYY-MM-DD for a local date. */
export const day = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

/** This month up to today: the default period (the server's default too). */
export function thisMonth(now = new Date()): Period {
  return { from: day(new Date(now.getFullYear(), now.getMonth(), 1)), to: day(now) };
}

/** The period in the URL (?from=&to=), defaulting to this month. */
export function usePeriod() {
  const url = useUrlState();
  const fallback = useMemo(() => thisMonth(), []);
  const period: Period = { from: url.get("from") ?? fallback.from, to: url.get("to") ?? fallback.to };
  return { period, set: (p: Period) => url.set({ from: p.from, to: p.to, page: undefined }) };
}

function useCountry() {
  const { scope } = useScope();
  return scope.country || undefined;
}

export function useAccounts(period: Period) {
  const country = useCountry();
  const query = { country, from: period.from, to: period.to };
  return useQuery({
    queryKey: ["admin", "finance", "accounts", query],
    queryFn: async () => unwrap(await api.GET("/admin/finance/accounts", { params: { query } })),
  });
}

export function useAccountEntries(key: string, period: Period, offset: number) {
  const country = useCountry();
  const query = { country, from: period.from, to: period.to, limit: ENTRIES_PAGE, offset };
  return useQuery({
    queryKey: ["admin", "finance", "accounts", key, "entries", query],
    queryFn: async () =>
      unwrap(await api.GET("/admin/finance/accounts/{key}/entries", { params: { path: { key }, query } })),
    placeholderData: keepPreviousData,
  });
}

/** Downloads the period's entries of an account as a CSV file. */
export function useExportAccount() {
  const country = useCountry();
  return useMutation({
    mutationFn: async ({ key, period }: { key: string; period: Period }) => {
      const r = await api.GET("/admin/finance/accounts/{key}/export", {
        params: { path: { key }, query: { country, from: period.from, to: period.to } },
        parseAs: "blob",
      });
      if (r.error || !r.data) {
        const e = (r.error as { error?: { message?: string; code?: string } } | undefined)?.error;
        throw new ApiError(e?.message ?? "Export failed", e?.code, r.response.status);
      }
      const url = URL.createObjectURL(r.data as Blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `${key.replace(":", "-").toLowerCase()}_${period.from}_${period.to}.csv`;
      link.click();
      URL.revokeObjectURL(url);
    },
  });
}

/** Unpaid rider refunds and riders by what they are owed (the rider-refunds payment). */
export function useOwedRefunds(enabled: boolean) {
  const country = useCountry();
  return useQuery({
    queryKey: ["admin", "finance", "accounts", "owed-refunds", country],
    queryFn: async () =>
      unwrap(await api.GET("/admin/finance/rider-refunds/owed", { params: { query: { country } } })),
    enabled,
  });
}

export interface PaymentDraft {
  key: string;
  idem: string; // Idempotency-Key: one per dialog opening, so a double click cannot pay twice
  amount: string;
  paid_on: string;
  method: Method;
  reference: string;
  reason: string;
  refund_ids?: string[];
  rider_id?: string;
}

/** Records a payment (audited); every finance view refreshes afterwards. */
export function useRecordPayment() {
  const qc = useQueryClient();
  const country = useCountry();
  return useMutation({
    mutationFn: async ({ key, idem, ...body }: PaymentDraft) =>
      unwrap(
        await api.POST("/admin/finance/accounts/{key}/payments", {
          params: { path: { key }, query: { country }, header: { "Idempotency-Key": idem } },
          body,
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "finance"] }),
  });
}
