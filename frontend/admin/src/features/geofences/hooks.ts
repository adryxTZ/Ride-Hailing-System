"use client";
/** Geofence hooks — the list, one geofence, overlaps, and every staff change. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "@/lib/api";

const KEY = ["admin", "geofences"];
export type Kind = "DISTRICT" | "ZONE" | "PLACE";
export type Shape = Schemas["GeofenceShape"];

export function useGeofences(country: string) {
  return useQuery({
    queryKey: [...KEY, "list", country],
    queryFn: async () => unwrap(await api.GET("/admin/geofences", { params: { query: { country } } })).data,
    enabled: !!country,
  });
}

export function useGeofence(id?: string) {
  return useQuery({
    queryKey: [...KEY, "one", id],
    queryFn: async () =>
      unwrap(await api.GET("/admin/geofences/{id}", { params: { path: { id: id as string } } })),
    enabled: !!id,
  });
}

export function useOverlaps(country: string) {
  return useQuery({
    queryKey: [...KEY, "overlaps", country],
    queryFn: async () =>
      unwrap(await api.GET("/admin/geofences/overlaps", { params: { query: { country } } })).data,
    enabled: !!country,
  });
}

function useWrite<T, R>(fn: (v: T) => Promise<R>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: KEY });
      void qc.invalidateQueries({ queryKey: ["admin", "profiles"] });
    },
  });
}

export const useCreateGeofence = () =>
  useWrite(async (b: Shape & { country: string; kind: Kind; code: string; name: string; reason: string }) =>
    unwrap(await api.POST("/admin/geofences", { body: b })),
  );

export const useReshape = () =>
  useWrite(async (a: { id: string; shape: Shape; reason: string }) =>
    unwrap(
      await api.PUT("/admin/geofences/{id}/geometry", {
        params: { path: { id: a.id } },
        body: { ...a.shape, reason: a.reason },
      }),
    ),
  );

export const useUpdateGeofence = () =>
  useWrite(
    async (a: { id: string; name?: string; status?: "ACTIVE" | "INACTIVE" | "ARCHIVED"; reason: string }) =>
      unwrap(
        await api.PATCH("/admin/geofences/{id}", {
          params: { path: { id: a.id } },
          body: { name: a.name, status: a.status, reason: a.reason },
        }),
      ),
  );

export const useImportGeofences = () =>
  useWrite(
    async (b: {
      country: string;
      kind: Kind;
      features: Record<string, unknown>;
      dry_run: boolean;
      reason?: string;
    }) => unwrap(await api.POST("/admin/geofences/import", { body: b })),
  );
