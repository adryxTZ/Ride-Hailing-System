/** How the rider's trip screen reads a trip status: one phase per distinct screen state. */
import type { Trip } from "@/services/trips";

export type TripPhase =
  | "searching"
  | "assigned"
  | "arrived"
  | "riding"
  | "paying"
  | "done"
  | "no_driver"
  | "cancelled"
  | "no_show";

const PHASE: Record<string, TripPhase> = {
  REQUESTED: "searching",
  OFFERED: "searching",
  ACCEPTED: "assigned",
  EN_ROUTE_TO_PICKUP: "assigned",
  ARRIVED_AT_PICKUP: "arrived",
  IN_TRANSIT: "riding",
  ARRIVED_AT_DROPOFF: "riding",
  AWAITING_PAYMENT: "paying",
  COMPLETED: "done",
  NO_DRIVER_FOUND: "no_driver",
  EXPIRED: "no_driver",
  CANCELLED_BY_RIDER: "cancelled",
  CANCELLED_BY_STAFF: "cancelled",
  CANCELLED_BY_DRIVER: "searching", // the server re-dispatches; the rider keeps waiting
};

export function phaseOf(trip?: Trip | null): TripPhase {
  // A driver's cancel normally sends the trip back to matching; a no-show ends it.
  if (trip?.status === "CANCELLED_BY_DRIVER" && trip.cancel_reason === "rider_no_show") return "no_show";
  return (trip && PHASE[trip.status]) || "searching";
}

/** The rider may cancel until the driver has picked them up. */
export function canCancel(phase: TripPhase): boolean {
  return phase === "searching" || phase === "assigned" || phase === "arrived";
}
