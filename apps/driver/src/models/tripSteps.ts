/**
 * What the driver sees at each trip status: the headline, where they are heading,
 * and the label of the one main button (the server's next non-cancel transition).
 */
import { PARCEL_SERVICE_KEY } from "@/config";
import type { Trip, TripStatus } from "@/services/trips";

interface Step {
  title: string;
  /** Which stop the map and navigation point at. */
  target: "PICKUP" | "DROPOFF";
}

const STEPS: Partial<Record<TripStatus, Step>> = {
  ACCEPTED: { title: "Head to pickup", target: "PICKUP" },
  EN_ROUTE_TO_PICKUP: { title: "Head to pickup", target: "PICKUP" },
  ARRIVED_AT_PICKUP: { title: "Waiting for the rider", target: "PICKUP" },
  IN_TRANSIT: { title: "Drive to destination", target: "DROPOFF" },
  ARRIVED_AT_DROPOFF: { title: "At destination", target: "DROPOFF" },
  AWAITING_PAYMENT: { title: "Collect payment", target: "DROPOFF" },
};

const ACTION_LABEL: Partial<Record<TripStatus, string>> = {
  EN_ROUTE_TO_PICKUP: "Start driving to pickup",
  ARRIVED_AT_PICKUP: "I've arrived",
  IN_TRANSIT: "Start trip",
  ARRIVED_AT_DROPOFF: "Arrived at destination",
  AWAITING_PAYMENT: "End trip",
  COMPLETED: "Cash collected",
};

/** Package delivery wording where it differs from a ride. */
const PACKAGE_STEPS: Partial<Record<TripStatus, Step>> = {
  ARRIVED_AT_PICKUP: { title: "Collect the package", target: "PICKUP" },
  IN_TRANSIT: { title: "Deliver the package", target: "DROPOFF" },
  ARRIVED_AT_DROPOFF: { title: "Hand over the package", target: "DROPOFF" },
};

const PACKAGE_ACTION_LABEL: Partial<Record<TripStatus, string>> = {
  IN_TRANSIT: "Package collected",
  ARRIVED_AT_DROPOFF: "At the receiver",
  AWAITING_PAYMENT: "Delivered",
};

/** True for a package delivery (not a ride). */
export const isPackage = (trip: { service: string }) => trip.service === PARCEL_SERVICE_KEY;

export function stepFor(trip: Trip): Step {
  const pkg = isPackage(trip) ? PACKAGE_STEPS[trip.status] : undefined;
  return pkg ?? STEPS[trip.status] ?? { title: "Trip", target: "PICKUP" };
}

/** The next forward move and its button label, or undefined when there is none. */
export function nextAction(trip: Trip): { to: TripStatus; label: string } | undefined {
  const to = trip.allowed_transitions.find((s) => !s.startsWith("CANCELLED"));
  if (!to) return undefined;
  const pkg = isPackage(trip) ? PACKAGE_ACTION_LABEL[to] : undefined;
  return { to, label: pkg ?? ACTION_LABEL[to] ?? to };
}

export const canDriverCancel = (trip: Trip) => trip.allowed_transitions.includes("CANCELLED_BY_DRIVER");

/** Statuses where the car is moving towards a stop (the turn-by-turn banner shows). */
const DRIVING: TripStatus[] = ["ACCEPTED", "EN_ROUTE_TO_PICKUP", "IN_TRANSIT"];

export function isDriving(trip: Trip): boolean {
  return DRIVING.includes(trip.status);
}

/** The person the driver deals with at a stop: who to show and call. */
export interface StopParty {
  role: string;
  name: string;
  phone?: string;
  /** "Booked by …" when the person is not the account holder. */
  bookedBy?: string;
}

/**
 * At the pickup: the passenger (or package sender) the rider named, else the rider.
 * At the drop-off: the package receiver when there is one, else the rider.
 */
export function partyFor(trip: Trip, target: "PICKUP" | "DROPOFF"): StopParty {
  const rider = trip.rider?.name ?? "Rider";
  const contact = target === "PICKUP" ? trip.contacts?.pickup : trip.contacts?.dropoff;
  if (!contact) return { role: isPackage(trip) ? "Sender" : "Rider", name: rider, phone: trip.rider?.phone };
  return {
    role: target === "DROPOFF" ? "Receiver" : isPackage(trip) ? "Sender" : "Passenger",
    name: contact.name,
    phone: contact.phone,
    bookedBy: `Booked by ${rider}`,
  };
}
