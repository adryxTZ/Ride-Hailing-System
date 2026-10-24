/** Words for resolution outcomes, scope kinds and notes in the pricing history screens. */
export const OUTCOME_LABEL: Record<string, string> = {
  WON: "Applied",
  LOWER_PRECEDENCE: "Outranked",
  INACTIVE: "Switched off",
  DRAFT: "Draft",
  OUT_OF_WINDOW: "Outside its dates",
  NO_VERSION_IN_FORCE: "No version yet",
  TIE_BROKEN: "Tie, older applied",
};

export const OUTCOME_TONE: Record<string, "ok" | "muted" | "warn"> = {
  WON: "ok",
  LOWER_PRECEDENCE: "muted",
  INACTIVE: "muted",
  DRAFT: "muted",
  OUT_OF_WINDOW: "warn",
  NO_VERSION_IN_FORCE: "warn",
  TIE_BROKEN: "warn",
};

export const KIND_LABEL: Record<string, string> = {
  COUNTRY: "Country",
  AREA: "Region",
  DISTRICT: "District",
  ZONE: "Zone",
  PLACE: "Place",
};

export const EVENT_LABEL: Record<string, string> = {
  CREATED: "Created",
  PUBLISHED: "Version published",
  ACTIVATED: "Switched on",
  DEACTIVATED: "Switched off",
  ASSIGNED: "Applied to a place",
  UNASSIGNED: "Removed from a place",
  MADE_DEFAULT: "Made the default",
  DEFAULT_REPLACED: "No longer the default",
  RENAMED: "Renamed",
  WINDOW_CHANGED: "Dates changed",
  MIGRATED: "Carried over from the old fee policy",
};

export const MATCHED_LABEL: Record<string, string> = {
  PICKUP: "Pickup",
  DROPOFF: "Drop-off",
  ROUTE: "On the route",
};
