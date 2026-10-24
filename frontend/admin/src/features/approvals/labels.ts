/** Approvals labels — words for statuses, step modes, error codes and diff fields. */
import type { Tone } from "@/components/ui/Chip";
import type { ApprovalRequest } from "./hooks";

type Status = ApprovalRequest["status"];

export const STATUS_LABEL: Record<Status, string> = {
  PENDING: "Waiting",
  APPROVED: "Approved",
  EXECUTING: "Applying",
  EXECUTED: "Applied",
  FAILED: "Approved, not applied",
  REJECTED: "Rejected",
  EXPIRED: "Expired",
  CANCELLED: "Withdrawn",
};

export const STATUS_TONE: Record<Status, Tone> = {
  PENDING: "warn",
  APPROVED: "live",
  EXECUTING: "live",
  EXECUTED: "ok",
  FAILED: "bad",
  REJECTED: "bad",
  EXPIRED: "muted",
  CANCELLED: "muted",
};

export const MODE_LABEL: Record<"ANY" | "ALL", string> = {
  ANY: "Any of these roles",
  ALL: "One from each role",
};

/** Why an approved change did not apply (the stored error code), in plain words. */
export const FAILURE_LABEL: Record<string, string> = {
  REFUND_TOO_LARGE: "The trip no longer has that much left to refund.",
  TRIP_NOT_REFUNDABLE: "The trip can no longer be refunded.",
  PAYOUT_NOT_ACTIONABLE: "The payout was already decided or cancelled.",
  PAYOUT_ABOVE_BALANCE: "The driver's balance is now below the payout.",
  PRIVACY_REQUEST_NOT_OPEN: "The privacy request was already handled.",
  DELETE_BLOCKED: "Something still blocks the account deletion.",
  THEME_NOT_DRAFT: "The theme is no longer a draft.",
  APPROVAL_ACTION_UNAVAILABLE: "This action is no longer available.",
  INTERNAL: "Something went wrong while applying it. Try the change again.",
};

/** "terms.booking_fee" → "Terms › Booking fee". */
export function fieldLabel(path: string): string {
  return path
    .split(".")
    .map((p) => {
      const words = p.replace(/_/g, " ");
      return words.charAt(0).toUpperCase() + words.slice(1);
    })
    .join(" › ");
}

/** "perm:trips.refund" → "Permission trips.refund"; a role reads as itself. */
export function memberLabel(member: string): string {
  return member.startsWith("perm:") ? `Permission ${member.slice("perm:".length)}` : member;
}
