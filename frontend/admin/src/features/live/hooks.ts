"use client";
/** Live map data, refreshed every 5 seconds. */
import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";
import { useScope } from "@/lib/scope";

const LIVE_POLL_MS = 5_000;

export function useLive() {
  const { query } = useScope();
  return useQuery({
    queryKey: ["admin", "live", query],
    queryFn: async () => unwrap(await api.GET("/admin/live", { params: { query } })),
    refetchInterval: LIVE_POLL_MS,
  });
}
