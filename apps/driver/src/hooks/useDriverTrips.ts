/**
 * Driver trip hooks — the open offer and the assigned trip, kept live by the
 * socket (offer.new / offer.closed / trip.updated) with slow polling as a fallback.
 */
import { useSession } from "@platform/auth";
import { appendMessage } from "@platform/chat";
import { type RealtimeFrame, useRealtime } from "@platform/realtime";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { chatKey } from "@/hooks/useChat";
import { NEXT_TRIP_KEY } from "@/hooks/useNextTrip";
import { isQueuedTrip } from "@/models/tripQueue";
import type { ChatPage, Message } from "@/services/messaging";
import { freshToken, REALTIME_URL } from "@/services/realtime";
import {
  acceptOffer,
  declineOffer,
  fetchActiveTrip,
  fetchOffer,
  fetchTripHistory,
  type Offer,
  type Trip,
  type TripStatus,
  transitionTrip,
} from "@/services/trips";
import { fetchWallet } from "@/services/wallet";

const OFFER_KEY = ["driver", "offer"] as const;
const TRIP_KEY = ["driver", "trip"] as const;
/** The last completed trip, waiting for the driver to rate the rider. */
export const RATE_KEY = ["driver", "rate-pending"] as const;
const FALLBACK_POLL_MS = 10_000;
const TERMINAL = [
  "COMPLETED",
  "CANCELLED_BY_RIDER",
  "CANCELLED_BY_DRIVER",
  "CANCELLED_BY_STAFF",
  "NO_DRIVER_FOUND",
  "EXPIRED",
];

/** The driver's socket; mount once (HomeScreen). */
export function useDriverRealtime() {
  const { isSignedIn } = useSession();
  const qc = useQueryClient();
  return useRealtime({
    url: REALTIME_URL,
    enabled: isSignedIn,
    getToken: freshToken,
    onFrame: (f: RealtimeFrame) => {
      if (f.type === "ready") {
        void qc.invalidateQueries({ queryKey: OFFER_KEY });
        void qc.invalidateQueries({ queryKey: TRIP_KEY });
      } else if (f.type === "offer.new") qc.setQueryData(OFFER_KEY, f.data as Offer);
      else if (f.type === "offer.closed") {
        const closed = (f.data as { trip_id: string }).trip_id;
        qc.setQueryData<Offer | null>(OFFER_KEY, (o) => (o?.trip_id === closed ? null : o));
      } else if (f.type === "trip.updated") {
        const t = f.data as Trip;
        void qc.invalidateQueries({ queryKey: NEXT_TRIP_KEY });
        // The queued next trip never replaces the current one; it arrives un-queued once current.
        if (isQueuedTrip(t)) return;
        // A trip that ended, or went back to matching after this driver cancelled, leaves the screen.
        const mine = !TERMINAL.includes(t.status) && t.status !== "REQUESTED" && t.status !== "OFFERED";
        qc.setQueryData<Trip | null>(TRIP_KEY, (cur) => (mine ? t : cur && cur.id !== t.id ? cur : null));
        if (t.status === "COMPLETED") qc.setQueryData(RATE_KEY, t);
      } else if (f.type === "wallet.updated") {
        // Balance, holds or a payment changed: re-read the wallet and the online status (holds gate it).
        void qc.invalidateQueries({ queryKey: ["driver", "wallet"] });
        void qc.invalidateQueries({ queryKey: ["driver", "status"] });
      } else if (f.type === "chat.message") {
        const m = f.data as Message;
        qc.setQueryData<ChatPage>(chatKey(m.trip_id), (page) => ({
          data: appendMessage(page?.data, m),
          quick_replies: page?.quick_replies ?? [],
        }));
      }
    },
  });
}

export function useOffer() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: OFFER_KEY,
    queryFn: ({ signal }) => fetchOffer(signal),
    enabled: isSignedIn,
    refetchInterval: FALLBACK_POLL_MS,
  });
}

export function useDriverTrip() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: TRIP_KEY,
    queryFn: ({ signal }) => fetchActiveTrip(signal),
    enabled: isSignedIn,
    refetchInterval: FALLBACK_POLL_MS,
  });
}

/** Accept, then start driving to the pickup straight away (one tap for the driver). */
export function useAcceptOffer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (tripId: string) => {
      const t = await acceptOffer(tripId);
      return t.allowed_transitions.includes("EN_ROUTE_TO_PICKUP")
        ? transitionTrip(t.id, "EN_ROUTE_TO_PICKUP")
        : t;
    },
    onSuccess: (t) => {
      qc.setQueryData(OFFER_KEY, null);
      // A queued next trip waits behind the current one: only the banner learns of it.
      if (isQueuedTrip(t)) void qc.invalidateQueries({ queryKey: NEXT_TRIP_KEY });
      else qc.setQueryData(TRIP_KEY, t);
    },
    onError: () => void qc.invalidateQueries({ queryKey: OFFER_KEY }),
  });
}

export function useDeclineOffer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: declineOffer,
    onSettled: () => qc.setQueryData(OFFER_KEY, null),
  });
}

export function useTransition() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, to, reason }: { id: string; to: TripStatus; reason?: "rider_no_show" }) =>
      transitionTrip(id, to, reason),
    onSuccess: (t) => {
      const ended = TERMINAL.includes(t.status) || t.status === "REQUESTED";
      const current = qc.getQueryData<Trip | null>(TRIP_KEY);
      // A step on the queued next trip (e.g. cancelling it) leaves the current trip alone.
      if (isQueuedTrip(t) || (current && current.id !== t.id))
        return void qc.invalidateQueries({ queryKey: NEXT_TRIP_KEY });
      qc.setQueryData(TRIP_KEY, ended ? null : t);
      if (t.status === "COMPLETED") qc.setQueryData(RATE_KEY, t);
      // A queued next trip becomes current when this one ends: read it back.
      if (ended) void qc.invalidateQueries({ queryKey: TRIP_KEY });
    },
  });
}

/** The driver's trips, newest first; refetched when a trip ends. */
export function useTripHistory() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["driver", "history"],
    queryFn: ({ signal }) => fetchTripHistory(signal),
    enabled: isSignedIn,
    staleTime: 30_000,
  });
}

/** The driver's wallet (balance and history); refetched when a trip ends. */
export function useWallet() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["driver", "wallet"],
    queryFn: ({ signal }) => fetchWallet(signal),
    enabled: isSignedIn,
    staleTime: 15_000,
  });
}

/** The completed trip whose rider is still to be rated (null once rated or skipped). */
export function useRatePending() {
  const qc = useQueryClient();
  const { data } = useQuery<Trip | null>({
    queryKey: RATE_KEY,
    queryFn: () => null,
    staleTime: Number.POSITIVE_INFINITY,
  });
  return { trip: data ?? undefined, clear: () => qc.setQueryData(RATE_KEY, null) };
}
