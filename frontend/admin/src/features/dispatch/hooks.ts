"use client";
/** Dispatch settings hooks — each country's trip-queue switches, and the audited change. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";

const KEY = ["admin", "dispatch", "settings"];

export type DispatchSettings = Schemas["DispatchSettings"];
export type FinishingStatus = DispatchSettings["finishing_statuses"][number];

export function useDispatchSettings() {
  return useQuery({
    queryKey: KEY,
    queryFn: async () => unwrap(await api.GET("/admin/dispatch/settings")).data,
  });
}

export interface DispatchChange {
  country: string;
  trip_queue_enabled?: boolean;
  finishing_statuses?: FinishingStatus[];
  reason: string;
}

export function useSetDispatchSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ country, ...body }: DispatchChange) =>
      unwrap(await api.PATCH("/admin/dispatch/settings/{country}", { params: { path: { country } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}
