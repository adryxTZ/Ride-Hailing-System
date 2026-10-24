/**
 * Words for finance accounts: who each account is, which way its balance
 * points ("We owe", "Owed to us", "Earned") and how payments are made.
 */
import type { Schemas } from "@/lib/api";
import { cents } from "@/lib/format";

export type Account = Schemas["FinanceAccount"];
export type Position = Account["position"];

/** The party an account kind stands for. `levy` is the country's regulator levy label. */
export function accountName(a: Pick<Account, "kind" | "name">, levy: string): string {
  switch (a.kind) {
    case "PLATFORM_REVENUE":
      return "Platform revenue";
    case "VAT_PAYABLE":
      return "VAT";
    case "LEVY_PAYABLE":
      return levy;
    case "CHARGES_PAYABLE":
      return a.name ?? "Charge payee";
    case "DRIVER_WALLET":
      return "Drivers";
    case "RIDER_DUES":
      return "Rider dues";
    case "RIDER_REFUNDS":
      return "Rider refunds";
    case "GATEWAY_CLEARING":
      return "Payment gateway";
    case "PLATFORM_BANK":
      return "Platform bank";
    default:
      return a.kind;
  }
}

/** Who the money is with, under the name. */
export const ACCOUNT_NOTE: Record<string, string> = {
  PLATFORM_REVENUE: "Commission and fees after tax",
  VAT_PAYABLE: "Owed to the tax authority",
  LEVY_PAYABLE: "Owed to the transport regulator",
  CHARGES_PAYABLE: "Third-party charges collected on fares",
  DRIVER_WALLET: "Cash-trip commission and earnings not yet paid out",
  RIDER_DUES: "Cancellation fees riders still owe",
  RIDER_REFUNDS: "Refunds and late fare payments owed back to riders",
  GATEWAY_CLEARING: "Mobile money collected, not yet in the bank",
  PLATFORM_BANK: "Payments recorded here (negative = paid out)",
};

/** Payee kinds, for the line under a payee's name. */
export const PAYEE_KIND: Record<string, string> = {
  TAX_AUTHORITY: "Tax authority",
  REGULATOR: "Regulator",
  AIRPORT: "Airport",
  MUNICIPALITY: "Municipality",
  PARKING_OPERATOR: "Parking operator",
  OTHER: "Third party",
};

/** The position in plain words; a negative balance reads the other way. */
export function positionLabel(position: Position, balance: number): string {
  const flipped = balance < 0;
  switch (position) {
    case "EARNED":
      return flipped ? "Net refunded" : "Earned";
    case "WE_OWE":
      return flipped ? "Owed to us" : "We owe";
    case "OWED_TO_US":
      return flipped ? "We owe" : "Owed to us";
    case "HELD":
      return "Held at gateway";
    default:
      return flipped ? "Paid out" : "In the bank";
  }
}

/** Chip tone per position word. */
export function positionTone(label: string): "ok" | "warn" | "brand" | "muted" | "live" {
  if (label === "Earned") return "ok";
  if (label === "We owe") return "warn";
  if (label === "Owed to us") return "live";
  return "muted";
}

/** Payment methods staff can record. */
export const METHODS = [
  { value: "BANK_TRANSFER", label: "Bank transfer" },
  { value: "MOBILE_MONEY", label: "Mobile money" },
  { value: "CASH", label: "Cash" },
  { value: "CHEQUE", label: "Cheque" },
  { value: "OTHER", label: "Other" },
] as const;
export type Method = (typeof METHODS)[number]["value"];

/** Ledger transaction kinds as staff read them in an account's entries. */
export const ENTRY_KIND: Record<string, string> = {
  "trip.completed": "Trip fare",
  "trip.cancellation_fee": "Cancellation fee",
  "trip.dues_collected": "Dues collected",
  "trip.refund": "Refund",
  "fare.unapplied": "Late fare payment",
  "payin.collected": "Driver debt paid",
  "payout.paid": "Driver cash-out",
  "finance.settlement": "Payment recorded",
};

/** "1.80%" from a rate string like "0.0180". */
export function rate(r?: string | null): string {
  if (r === undefined || r === null) return "—";
  return `${(Number(r) * 100).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}%`;
}

/** "TZS 17,847.10": ledger amounts are shown exactly, with cents. */
export function amount(v: string | number | null | undefined, currency: string): string {
  return `${currency} ${cents(v)}`;
}
