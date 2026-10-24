/** Plain words for privacy request kinds, statuses, blockers and retention rules. */

export const KIND_LABEL: Record<string, string> = {
  EXPORT: "Copy of their data",
  DELETE: "Delete account",
};

export const STATUS_LABEL: Record<string, string> = {
  REQUESTED: "To handle",
  IN_PROGRESS: "Building file",
  READY: "Ready to download",
  DONE: "Done",
  REJECTED: "Rejected",
  CANCELLED: "Cancelled by them",
};

export const STATUS_TONE: Record<string, "live" | "ok" | "muted" | "bad" | "warn"> = {
  REQUESTED: "live",
  IN_PROGRESS: "warn",
  READY: "ok",
  DONE: "ok",
  REJECTED: "bad",
  CANCELLED: "muted",
};

export const APP_LABEL: Record<string, string> = { RIDER: "Rider", DRIVER: "Driver" };

/** What each blocker means, and what staff do about it. */
export const BLOCKER_LABEL: Record<string, string> = {
  ACTIVE_TRIP: "They are on a trip right now",
  DRIVER_DEBT: "The driver owes the Platform money",
  DRIVER_BALANCE: "The driver has earnings not yet paid out",
  OPEN_PAYOUT: "A payout to the driver is still being sent",
  RIDER_DUES: "The rider has unpaid trips",
  OWED_TO_RIDER: "A refund or credit is still owed to the rider",
};

export const BLOCKER_FIX: Record<string, string> = {
  ACTIVE_TRIP: "Wait for the trip to end, or end it from the trip page.",
  DRIVER_DEBT: "Collect or settle the debt on the driver's page.",
  DRIVER_BALANCE: "Pay out the balance from the driver's page.",
  OPEN_PAYOUT: "Wait for the payout to finish or fail in Finance.",
  RIDER_DUES: "Collect or write off what they owe on the rider's page.",
  OWED_TO_RIDER: "Send the refund or credit from the rider's page.",
};

/** Where a blocker gets fixed: the person's page, or Finance for payouts. */
export const BLOCKER_PLACE: Record<string, "person" | "finance" | "trips"> = {
  ACTIVE_TRIP: "trips",
  DRIVER_DEBT: "person",
  DRIVER_BALANCE: "person",
  OPEN_PAYOUT: "finance",
  RIDER_DUES: "person",
  OWED_TO_RIDER: "person",
};

export const RETENTION_LABEL: Record<string, string> = {
  otp_codes: "Sign-in codes",
  sessions: "Signed-out sessions",
  trip_messages: "Trip chat messages",
  share_tokens: "Shared trip links",
  gps_evidence: "GPS evidence",
  fraud_flags_dismissed: "Dismissed fraud flags",
  devices_installs: "Old devices and app installs",
  regulator_payloads: "Regulator submissions",
  kyc_rejected: "Rejected driver applications",
  kyc_closed: "Closed driver accounts' documents",
  documents_archived: "Replaced documents",
  privacy_exports: "Data copies for download",
};
