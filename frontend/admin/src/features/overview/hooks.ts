"use client";
/** Overview data, refreshed every 30 s while the page is open. */
import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";
import { useScope } from "@/lib/scope";

const OVERVIEW_POLL_MS = 30_000;

export function useOverview() {
  const { query } = useScope();
  return useQuery({
    queryKey: ["admin", "overview", query],
    queryFn: async () => unwrap(await api.GET("/admin/overview", { params: { query } })),
    refetchInterval: OVERVIEW_POLL_MS,
  });
}
