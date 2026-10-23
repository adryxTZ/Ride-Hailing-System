"use client";
/** People hooks — the rider page, the driver activity, one person's trips, sessions, notes and staff actions. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";

export const PERSON_TRIPS_PAGE = 20;
/** Trips in progress refresh this often. */
const ACTIVE_POLL_MS = 10_000;
export type Role = "rider" | "driver";
export type TripGroup = "active" | "scheduled" | "completed" | "cancelled";

export function useRider(id: string) {
  return useQuery({
    queryKey: ["admin", "person", "rider", id],
    queryFn: async () => unwrap(await api.GET("/admin/riders/{id}", { params: { path: { id } } })),
  });
}

export function useDriverActivity(id: string) {
  return useQuery({
    queryKey: ["admin", "person", "driver", id],
    queryFn: async () => unwrap(await api.GET("/admin/drivers/{id}/activity", { params: { path: { id } } })),
  });
}

/** One person's trips, newest first (every country they rode or drove in). */
export function usePersonTrips(role: Role, id: string, status?: TripGroup, offset = 0, q?: string) {
  return useQuery({
    queryKey: ["admin", "person-trips", role, id, status, offset, q],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/trips", {
          params: {
            query: {
              ...(role === "rider" ? { rider_id: id } : { driver_id: id }),
              status,
              q: q || undefined,
              limit: PERSON_TRIPS_PAGE,
              offset,
            },
          },
        }),
      ),
    refetchInterval: status === "active" ? ACTIVE_POLL_MS : false,
    placeholderData: (prev) => prev,
  });
}

/** What staff did to this person (blocks, decisions, notes, ended sessions). */
export function usePersonLog(id: string) {
  return useQuery({
    queryKey: ["admin", "audit", "target", id],
    queryFn: async () =>
      unwrap(await api.GET("/admin/audit", { params: { query: { target: `user:${id}`, limit: 100 } } })).data,
  });
}

function useRefresh() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: ["admin"] });
}

export function useEndPersonSessions(role: Role) {
  const done = useRefresh();
  return useMutation({
    mutationFn: async (a: { id: string; sessionId?: string; reason: string }) => {
      const opts = { params: { path: { id: a.id } }, body: { session_id: a.sessionId, reason: a.reason } };
      return unwrap(
        role === "rider"
          ? await api.POST("/admin/riders/{id}/sessions/end", opts)
          : await api.POST("/admin/drivers/{id}/sessions/end", opts),
      );
    },
    onSuccess: done,
  });
}

export function useAddNote() {
  const done = useRefresh();
  return useMutation({
    mutationFn: async (a: { id: string; body: string }) =>
      unwrap(
        await api.POST("/admin/people/{id}/notes", {
          params: { path: { id: a.id } },
          body: { body: a.body },
        }),
      ),
    onSuccess: done,
  });
}

/** Block or unblock a rider (ends their sessions). */
export function useRiderStatus() {
  const done = useRefresh();
  return useMutation({
    mutationFn: async (a: { id: string; block: boolean; reason: string }) => {
      const opts = { params: { path: { id: a.id } }, body: { reason: a.reason } };
      return unwrap(
        a.block
          ? await api.POST("/admin/riders/{id}/block", opts)
          : await api.POST("/admin/riders/{id}/unblock", opts),
      );
    },
    onSuccess: done,
  });
}

/** The Charts tab's series over the last `days` days (fetched only when the tab opens). */
export function usePersonCharts(role: Role, id: string, days: number, enabled: boolean) {
  return useQuery({
    queryKey: ["admin", "person-charts", role, id, days],
    queryFn: async () => {
      const opts = { params: { path: { id }, query: { days } } };
      return unwrap(
        role === "rider"
          ? await api.GET("/admin/riders/{id}/charts", opts)
          : await api.GET("/admin/drivers/{id}/charts", opts),
      );
    },
    enabled,
    placeholderData: (prev) => prev,
  });
}
