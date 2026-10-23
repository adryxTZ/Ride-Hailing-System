/**
 * bookingStore — the in-progress booking draft shared across the booking
 * screens (service, pickup, destination, attributes, quote, chosen option).
 * Cleared when a trip is created or the flow is abandoned.
 */
import { create } from "zustand";
import { SERVICE_KEY } from "@/config";
import type { Point, Quote, TripCreate } from "@/services/trips";

/** Passenger/sender and receiver when they are not the account holder. */
export type TripContacts = NonNullable<TripCreate["contacts"]>;

export interface PlacePick {
  point: Point;
  address: string;
  /** Where it came from: the phone's GPS, a saved place, search/recents, or the map pin. */
  source?: "gps" | "saved" | "search" | "map";
}

interface BookingState {
  serviceKey?: string;
  pickup?: PlacePick;
  dropoff?: PlacePick;
  /** Stops between pickup and drop-off, in order; undefined = an empty slot being filled. */
  stops: (PlacePick | undefined)[];
  attributes: Record<string, unknown>;
  contacts: TripContacts;
  quote?: Quote;
  vehicleClass?: string;
  paymentMethod: "CASH" | "MOBILE_MONEY" | "CARD" | "WALLET";
  /** ISO pickup time when booking ahead; undefined = ride now. */
  scheduledFor?: string;
  /** Note for the driver (for a package: what is being sent). */
  notes?: string;
  profile: "personal" | "work";
  start: (serviceKey?: string) => void;
  setPickup: (p: PlacePick) => void;
  setDropoff: (p: PlacePick) => void;
  addStop: () => void;
  setStop: (index: number, p: PlacePick) => void;
  removeStop: (index: number) => void;
  setAttributes: (a: Record<string, unknown>) => void;
  setContacts: (c: TripContacts) => void;
  setQuote: (q: Quote | undefined) => void;
  setVehicleClass: (k: string) => void;
  setPaymentMethod: (m: BookingState["paymentMethod"]) => void;
  setProfile: (p: BookingState["profile"]) => void;
  setScheduledFor: (iso?: string) => void;
  setNotes: (notes?: string) => void;
  reset: () => void;
}

const initial = {
  serviceKey: SERVICE_KEY as string | undefined,
  pickup: undefined,
  dropoff: undefined,
  stops: [] as (PlacePick | undefined)[],
  attributes: {},
  contacts: {} as TripContacts,
  quote: undefined,
  vehicleClass: undefined,
  paymentMethod: "CASH" as const,
  profile: "personal" as const,
  scheduledFor: undefined as string | undefined,
  notes: undefined as string | undefined,
};

/** Stops a ride may have between pickup and drop-off (server limit). */
export const MAX_STOPS = 3;

/** The filled stops, in order (empty slots are ignored when pricing and booking). */
export const filledStops = (stops: (PlacePick | undefined)[]): PlacePick[] =>
  stops.filter((x): x is PlacePick => !!x);

export const useBookingStore = create<BookingState>((set) => ({
  ...initial,
  start: (serviceKey = SERVICE_KEY) => set({ ...initial, serviceKey }),
  setPickup: (pickup) => set({ pickup, quote: undefined }),
  setDropoff: (dropoff) => set({ dropoff, quote: undefined }),
  addStop: () => set((st) => (st.stops.length >= MAX_STOPS ? st : { stops: [...st.stops, undefined] })),
  setStop: (index, p) =>
    set((st) => ({ stops: st.stops.map((x, i) => (i === index ? p : x)), quote: undefined })),
  removeStop: (index) => set((st) => ({ stops: st.stops.filter((_, i) => i !== index), quote: undefined })),
  setAttributes: (attributes) => set({ attributes, quote: undefined }),
  setContacts: (contacts) => set({ contacts }),
  setQuote: (quote) => set({ quote }),
  setVehicleClass: (vehicleClass) => set({ vehicleClass }),
  setPaymentMethod: (paymentMethod) => set({ paymentMethod }),
  setProfile: (profile) => set({ profile }),
  setScheduledFor: (scheduledFor) => set({ scheduledFor }),
  setNotes: (notes) => set({ notes }),
  // The payment method is the rider's preference, not part of one booking: it survives a reset.
  reset: () => set((s) => ({ ...initial, paymentMethod: s.paymentMethod })),
}));
