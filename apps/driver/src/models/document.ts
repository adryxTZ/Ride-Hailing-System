/**
 * Document model — expiry states in plain words, and the date rules the
 * expiry sheet checks before the server does (pure: tested in document.test.ts).
 */
import type { components } from "@platform/api-client";

export type DriverDocument = components["schemas"]["Document"];
export type Requirement = components["schemas"]["Requirement"];

export type ChipTone = "ok" | "warn" | "bad" | "muted";

/** The chip shown next to a document. */
export function expiryChip(doc?: DriverDocument): { text: string; tone: ChipTone } | undefined {
  if (!doc) return undefined;
  if (doc.renewal && doc.renewal.status === "PENDING") return { text: "Renewal under review", tone: "muted" };
  const left = doc.days_left ?? 0;
  switch (doc.expiry_state) {
    case "EXPIRED":
      return { text: "Expired", tone: "bad" };
    case "EXPIRING":
      return {
        text: left === 0 ? "Expires today" : left === 1 ? "Expires tomorrow" : `Expires in ${left} days`,
        tone: "warn",
      };
    case "VALID":
      return { text: `Valid until ${doc.expires_on}`, tone: "ok" };
    case "UNKNOWN":
      return { text: "Add the expiry date", tone: "muted" };
  }
  return undefined;
}

/** Why a typed expiry date is not accepted ("" when it is): YYYY-MM-DD, not in the past, at most 10 years ahead. */
export function expiryProblem(value: string, today = new Date()): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return "Use YYYY-MM-DD, as printed on the document.";
  const d = new Date(`${value}T00:00:00Z`);
  if (Number.isNaN(d.getTime())) return "That date does not exist.";
  const day = Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate());
  if (d.getTime() < day) return "This date has passed. Upload a document that is still valid.";
  if (d.getTime() > day + 10 * 365.25 * 86400000) return "That is more than 10 years away. Check the date.";
  return "";
}

/** Readable names for document codes in hold messages ("INSURANCE, LICENCE" → "insurance and licence"). */
export function kindsText(codes: string): string {
  const names = codes
    .split(",")
    .map((c) => c.trim().toLowerCase().replace(/_/g, " "))
    .filter(Boolean);
  return names.length > 1
    ? `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`
    : (names[0] ?? "documents");
}
