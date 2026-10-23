"use client";
/** Safety hooks — the SOS queue (polled), resolving an alert. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";

const SOS_POLL_MS = 15_000;

export function useSosAlerts(all = false) {
  return useQuery({
    queryKey: ["admin", "sos", all],
    queryFn: async () =>
      unwrap(await api.GET("/admin/sos", { params: { query: all ? { all: "1" } : {} } })).data,
    refetchInterval: SOS_POLL_MS,
  });
}

/** Number of open SOS alerts (badge on the bell and the rail). */
export function useOpenSos(): number {
  return useSosAlerts(false).data?.length ?? 0;
}

export function useResolveSos() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, resolution }: { id: string; resolution: string }) =>
      unwrap(await api.POST("/admin/sos/{id}/resolve", { params: { path: { id } }, body: { resolution } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "sos"] }),
  });
}
