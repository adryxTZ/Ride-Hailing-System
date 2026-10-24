/** Words for fraud rules, severities, statuses and self-booking signals. */
export const RULE_LABEL: Record<string, string> = {
  GPS_SPOOF: "Fake GPS",
  SELF_BOOKING: "Self-booking",
  DRIVER_CANCEL_RATE: "Driver cancels often",
  DRIVER_NO_SHOW_RATE: "Many rider no-shows",
  RIDER_CANCEL_RATE: "Rider cancels often",
};

export const RULE_HELP: Record<string, string> = {
  GPS_SPOOF:
    "The phone reported a mocked location, or the driver jumped further than a car can travel, several times within 10 minutes.",
  SELF_BOOKING:
    "The rider and driver look like the same person: they signed in from the same app install, or the passenger or receiver phone is the driver's.",
  DRIVER_CANCEL_RATE:
    "Over the last 7 days the driver cancelled at least 30% of the rides they accepted (at least 10 accepted).",
  DRIVER_NO_SHOW_RATE:
    "Over the last 7 days at least 30% of the driver's pickups ended as a rider no-show (at least 10 arrivals).",
  RIDER_CANCEL_RATE:
    "Over the last 7 days the rider cancelled at least 40% of rides after a driver was on the way (at least 10).",
};

export const SEVERITY_TONE: Record<string, "bad" | "warn" | "muted"> = {
  HIGH: "bad",
  MEDIUM: "warn",
  LOW: "muted",
};
export const SEVERITY_LABEL: Record<string, string> = { HIGH: "High", MEDIUM: "Medium", LOW: "Low" };

export const STATUS_LABEL: Record<string, string> = {
  OPEN: "To review",
  DISMISSED: "Dismissed",
  CONFIRMED: "Confirmed",
};
export const STATUS_TONE: Record<string, "live" | "muted" | "bad"> = {
  OPEN: "live",
  DISMISSED: "muted",
  CONFIRMED: "bad",
};

export const SIGNAL_LABEL: Record<string, string> = {
  SHARED_INSTALL: "Rider and driver signed in from the same app install",
  PASSENGER_IS_DRIVER: "The passenger or receiver phone is the driver's",
};
