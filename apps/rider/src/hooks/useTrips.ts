/**
 * Trip hooks — active trip and one trip by id (kept fresh by realtime frames, with
 * slow polling as a fallback), history, booking and cancel mutations.
 */
import { useSession } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  cancelTrip,
  createTrip,
  fetchActiveTrip,
  fetchDues,
  fetchTrip,
  fetchTrips,
  requestQuote,
  TERMINAL,
  type TripCreate,
} from "@/services/trips";

const ACTIVE_POLL_MS = 10_000; // fallback only: realtime frames update the cache immediately

export function useActiveTrip() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["trip", "active"],
    queryFn: ({ signal }) => fetchActiveTrip(signal),
    enabled: isSignedIn,
    refetchInterval: (q) => (q.state.data ? ACTIVE_POLL_MS : false),
  });
}

/** One trip by id, including terminal states (completion summary, "no driver"). */
export function useTrip(id?: string) {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["trip", id],
    queryFn: ({ signal }) => fetchTrip(id as string, signal),
    enabled: isSignedIn && !!id,
    refetchInterval: (q) => (q.state.data && TERMINAL.includes(q.state.data.status) ? false : ACTIVE_POLL_MS),
  });
}

export function useTripHistory() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["trips"],
    queryFn: ({ signal }) => fetchTrips(signal),
    enabled: isSignedIn,
    staleTime: 30_000,
  });
}

export function useQuote() {
  return useMutation({ mutationFn: requestQuote });
}

export function useCreateTrip() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: TripCreate) => createTrip(body),
    onSuccess: (trip) => {
      // A ride booked ahead is not in progress: it waits in Activity → Upcoming.
      if (trip.status !== "SCHEDULED") qc.setQueryData(["trip", "active"], trip);
      void qc.invalidateQueries({ queryKey: ["trips"] });
    },
  });
}

export function useCancelTrip() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, reason, note }: { id: string; reason: string; note?: string }) =>
      cancelTrip(id, reason, note),
    onSuccess: () => {
      qc.setQueryData(["trip", "active"], null);
      void qc.invalidateQueries({ queryKey: ["trips"] });
    },
  });
}

/** Earlier cancellation fees still owed (refreshed when trips change). */
export function useDues() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["trips", "dues"],
    queryFn: ({ signal }) => fetchDues(signal),
    enabled: isSignedIn,
    staleTime: 30_000,
  });
}
