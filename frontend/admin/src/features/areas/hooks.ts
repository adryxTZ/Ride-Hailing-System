"use client";
/** Areas hooks — list for a country, one area with its cells, and the audited writes. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";

const KEY = ["admin", "areas"];

export function useArea(id?: string) {
  return useQuery({
    queryKey: [...KEY, "one", id],
    queryFn: async () =>
      unwrap(await api.GET("/admin/areas/{id}", { params: { path: { id: id as string } } })),
    enabled: !!id,
  });
}

function useAreaWrite<T>(fn: (v: T) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({ mutationFn: fn, onSuccess: () => qc.invalidateQueries({ queryKey: KEY }) });
}

export const useCreateArea = () =>
  useAreaWrite(async (body: Schemas["AreaInput"]) => unwrap(await api.POST("/admin/areas", { body })));

export const useUpdateArea = () =>
  useAreaWrite(async (v: { id: string; body: Schemas["AreaUpdate"] }) =>
    unwrap(await api.PATCH("/admin/areas/{id}", { params: { path: { id: v.id } }, body: v.body })),
  );

export const useSaveCells = () =>
  useAreaWrite(async (v: { id: string; cells: string[]; reason: string }) =>
    unwrap(
      await api.PUT("/admin/areas/{id}/cells", {
        params: { path: { id: v.id } },
        body: { cells: v.cells, reason: v.reason },
      }),
    ),
  );
