/**
 * useTripRealtime — the rider's socket. trip.updated replaces the cached trip
 * (by id and as the active trip); driver.location moves the car on the map.
 * On (re)connect the caches are refetched because missed frames are not replayed.
 */
import { useSession } from "@platform/auth";
import { appendMessage } from "@platform/chat";
import { type RealtimeFrame, useRealtime } from "@platform/realtime";
import { useQueryClient } from "@tanstack/react-query";
import { chatKey } from "@/hooks/useChat";
import type { ChatPage, Message } from "@/services/messaging";
import { freshToken, REALTIME_URL } from "@/services/realtime";
import { type DriverLocation, TERMINAL, type Trip } from "@/services/trips";

type LocationFrame = { trip_id: string } & DriverLocation;

export function useTripRealtime() {
  const { isSignedIn } = useSession();
  const qc = useQueryClient();
  const status = useRealtime({
    url: REALTIME_URL,
    enabled: isSignedIn,
    getToken: freshToken,
    onFrame: (f: RealtimeFrame) => {
      if (f.type === "ready") return void qc.invalidateQueries({ queryKey: ["trip"] });
      if (f.type === "trip.updated") return applyTrip(f.data as Trip);
      if (f.type === "driver.location") return applyLocation(f.data as LocationFrame);
      if (f.type === "chat.message") return applyChat(f.data as Message);
    },
  });

  function applyTrip(t: Trip) {
    const prev = qc.getQueryData<Trip>(["trip", t.id]);
    const merged = { ...t, driver_location: t.driver_location ?? prev?.driver_location };
    qc.setQueryData(["trip", t.id], merged);
    // Rides booked ahead are not in progress; ending a ride clears the active slot.
    if (t.status !== "SCHEDULED")
      qc.setQueryData(["trip", "active"], TERMINAL.includes(t.status) ? null : merged);
    else void qc.invalidateQueries({ queryKey: ["trips"] });
    if (TERMINAL.includes(t.status)) void qc.invalidateQueries({ queryKey: ["trips"] });
  }

  function applyChat(m: Message) {
    qc.setQueryData<ChatPage>(chatKey(m.trip_id), (page) => ({
      data: appendMessage(page?.data, m),
      quick_replies: page?.quick_replies ?? [],
    }));
  }

  function applyLocation(l: LocationFrame) {
    const loc: DriverLocation = { location: l.location, heading: l.heading, at: l.at };
    for (const key of [
      ["trip", l.trip_id],
      ["trip", "active"],
    ]) {
      qc.setQueryData<Trip | null>(key, (t) =>
        t && t.id === l.trip_id ? { ...t, driver_location: loc } : t,
      );
    }
  }

  return status;
}
