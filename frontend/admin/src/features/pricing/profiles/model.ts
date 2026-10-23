/** Profile rules as fields, charge choices and their words: shared by the profile screens. */
import type { Schemas } from "@/lib/api";

export type Terms = Schemas["ProfileTerms"];
export type TermKey = keyof Terms;
export type Charge = Schemas["ChargeRule"];

/** How a rule is edited: a percentage of a fraction, an amount, seconds shown as minutes, a choice, yes/no. */
export type FieldKind = "pct" | "money" | "minutes" | "days" | "choice" | "bool";

export interface TermField {
  key: TermKey;
  label: string;
  kind: FieldKind;
  group: "fees" | "time" | "tax" | "debt" | "cashout";
  /** Set on the country default only; regions inherit it. */
  countryOnly?: boolean;
  options?: { value: string; label: string }[];
  hint?: string;
}

export const TERM_FIELDS: TermField[] = [
  { key: "commission_rate", label: "Commission", kind: "pct", group: "fees" },
  { key: "booking_fee", label: "Booking fee", kind: "money", group: "fees" },
  { key: "fare_rounding", label: "Round fares to", kind: "money", group: "fees", hint: "0 = no rounding" },
  {
    key: "waiting_cap_rate",
    label: "Waiting fee cap",
    kind: "pct",
    group: "time",
    hint: "Share of the fare; 0 = no cap",
  },
  { key: "cancel_free_s", label: "Free cancel window", kind: "minutes", group: "time" },
  { key: "no_show_after_s", label: "No-show after", kind: "minutes", group: "time" },
  { key: "cancel_driver_share", label: "Driver's share of cancel fees", kind: "pct", group: "time" },
  { key: "vat_rate", label: "VAT", kind: "pct", group: "tax", countryOnly: true },
  {
    key: "vat_base",
    label: "VAT charged on",
    kind: "choice",
    group: "tax",
    countryOnly: true,
    options: [
      { value: "COMMISSION", label: "Platform revenue" },
      { value: "GROSS_FARE", label: "Whole fare" },
    ],
  },
  { key: "levy_rate", label: "Levy", kind: "pct", group: "tax", countryOnly: true },
  {
    key: "levy_borne_by",
    label: "Levy paid by",
    kind: "choice",
    group: "tax",
    countryOnly: true,
    options: [
      { value: "PLATFORM", label: "Platform" },
      { value: "DRIVER", label: "Driver" },
      { value: "RIDER", label: "Rider" },
    ],
  },
  { key: "debt_soft_limit", label: "Soft debt limit", kind: "money", group: "debt", countryOnly: true },
  { key: "debt_hard_limit", label: "Hard debt limit", kind: "money", group: "debt", countryOnly: true },
  { key: "debt_soft_grace_days", label: "Grace days", kind: "days", group: "debt", countryOnly: true },
  { key: "payin_min_amount", label: "Minimum pay-in", kind: "money", group: "debt", countryOnly: true },
  { key: "payout_min_amount", label: "Minimum cash-out", kind: "money", group: "cashout", countryOnly: true },
  {
    key: "payout_auto_approve",
    label: "Cash-outs paid without approval",
    kind: "bool",
    group: "cashout",
    countryOnly: true,
  },
];

export const GROUP_TITLE: Record<TermField["group"], string> = {
  fees: "Fees",
  time: "Waiting and cancellations",
  tax: "Tax and levy (country)",
  debt: "Driver debt (country)",
  cashout: "Driver cash-outs (country)",
};

/** "15%", "TZS 500", "2 min", "Yes" for a rule value. */
export function showTerm(f: TermField, v: Terms[TermKey], currency = ""): string {
  if (v === null || v === undefined) return "Not set";
  switch (f.kind) {
    case "pct":
      return `${Math.round(Number(v) * 10000) / 100}%`;
    case "money":
      return `${currency} ${Number(v).toLocaleString("en-US")}`.trim();
    case "minutes":
      return `${Math.round(Number(v) / 60)} min`;
    case "days":
      return `${v} days`;
    case "bool":
      return v ? "Yes" : "No";
    default:
      return f.options?.find((o) => o.value === v)?.label ?? String(v);
  }
}

export const CATEGORY_LABEL: Record<string, string> = {
  TOLL: "Toll",
  PARKING: "Parking",
  LEVY: "Levy",
  AIRPORT_FEE: "Airport fee",
  ENTRY_FEE: "Entry fee",
  NIGHT: "Night",
  OTHER: "Other",
};
export const AMOUNT_TYPE_LABEL: Record<string, string> = {
  FLAT: "Fixed amount",
  PER_KM: "Per km",
  PERCENT_OF_FARE: "% of the fare",
};
export const RECIPIENT_LABEL: Record<string, string> = {
  FARE: "Part of the fare",
  PLATFORM: "Platform",
  DRIVER: "Driver keeps it",
  THIRD_PARTY: "Paid to a payee",
};
export const WHEN_LABEL: Record<string, string> = {
  PICKUP: "Pickup here",
  DROPOFF: "Drop-off here",
  EITHER: "Pickup or drop-off here",
  BOTH: "Pickup and drop-off here",
  ROUTE_CROSSES: "Route passes through",
};
/** Conditions not selectable yet (none: geofences exist). */
export const NEEDS_GEOFENCES = new Set<string>();
export const DAY_LABEL = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

export const STATUS_TONE = { ACTIVE: "ok", INACTIVE: "muted", DRAFT: "live" } as const;
export const STATUS_LABEL = { ACTIVE: "Active", INACTIVE: "Off", DRAFT: "Draft" } as const;

export const EVENT_LABEL: Record<string, string> = {
  CREATED: "Created",
  MIGRATED: "Moved from fee policies",
  PUBLISHED: "New version published",
  ACTIVATED: "Switched on",
  DEACTIVATED: "Switched off",
  ASSIGNED: "Applied to a place",
  UNASSIGNED: "Removed from a place",
  MADE_DEFAULT: "Made the default",
  DEFAULT_REPLACED: "Replaced as default",
  RENAMED: "Renamed",
  WINDOW_CHANGED: "Dates changed",
};

/** A blank charge to start the editor with. */
export const newCharge = (): Charge => ({
  code: "",
  label: { en: "", sw: "" },
  category: "OTHER",
  amount_type: "FLAT",
  amount: "0",
  recipient: "DRIVER",
  applies_when: "PICKUP",
  on_fixed_routes: true,
  services: [],
  vehicle_classes: [],
  days: [],
});

/** A percentage typed by staff as the stored fraction: "7.5" → "0.075" (4 decimals, no float noise). */
export function pctToFraction(text: string): string {
  const n = Number(text || 0) / 100;
  return String(Number(n.toFixed(4)));
}

/** The stored fraction as the percentage staff edit: "0.075" → "7.5". */
export const fractionToPct = (v?: string | number | null) =>
  v === null || v === undefined ? "" : String(Number((Number(v) * 100).toFixed(2)));
