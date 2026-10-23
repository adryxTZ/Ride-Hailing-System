/** useSafety — trusted contacts (cached) and the trip safety actions: share, SOS. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Location from "expo-location";
import { Share } from "react-native";
import {
  fetchTrustedContacts,
  raiseSos,
  saveTrustedContacts,
  shareTrip,
  type TrustedContact,
} from "@/services/safety";

const CONTACTS_KEY = ["me", "trusted-contacts"] as const;

export function useTrustedContacts() {
  return useQuery({ queryKey: CONTACTS_KEY, queryFn: ({ signal }) => fetchTrustedContacts(signal) });
}

export function useSaveTrustedContacts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (list: TrustedContact[]) => saveTrustedContacts(list),
    onSuccess: (list) => qc.setQueryData(CONTACTS_KEY, list),
  });
}

/** Opens the phone's share sheet with the trip's live link. */
export function useShareTrip() {
  return useMutation({
    mutationFn: async ({ tripId, driver }: { tripId: string; driver?: string }) => {
      const link = await shareTrip(tripId);
      await Share.share({
        message: `Follow my ride${driver ? ` with ${driver}` : ""} live: ${link.url}`,
        url: link.url,
      });
      return link;
    },
  });
}

/** Raises SOS with the phone's position when available. */
export function useSos() {
  return useMutation({
    mutationFn: async (tripId: string) => {
      const pos = await Location.getLastKnownPositionAsync().catch(() => null);
      return raiseSos(tripId, pos ? { lat: pos.coords.latitude, lng: pos.coords.longitude } : undefined);
    },
  });
}
