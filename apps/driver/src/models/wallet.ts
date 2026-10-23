/**
 * Wallet model — the driver's balance with the platform, the debt rule and
 * mobile-money payments, plus the words the screens use for them.
 */
import type { components, paths } from "@platform/api-client";
import { kindsText } from "@/models/document";
import { money } from "@/models/earnings";

export type Wallet = paths["/driver/wallet"]["get"]["responses"][200]["content"]["application/json"];
export type Hold = components["schemas"]["Hold"];
export type Payin = components["schemas"]["Payin"];
export type DebtLimits = components["schemas"]["DebtLimits"];

/** Where a payment stands, as the sheet shows it. */
export type PayinPhase = "idle" | "submitting" | "pending" | "success" | "failed";

/** What the driver owes in whole shillings (payments are whole), never negative. */
export const debtOf = (w: Wallet) => Math.max(0, Math.ceil(Number(w.debt ?? -Number(w.balance))));

/** The suggested payment: what lifts the hold, else the whole debt. */
export const suggestedPayment = (w: Wallet) => Math.ceil(Number(w.pay_to_unhold ?? debtOf(w)));

/** The hold that matters most to the driver: debt first (they can act on it), then staff. */
export const mainHold = (holds: Hold[] | undefined) => holds?.find((h) => h.kind === "DEBT") ?? holds?.[0];

/** The one-line title for a hold. */
export const holdTitle = (h: Hold) =>
  h.kind === "DEBT"
    ? "On hold: pay to go online"
    : h.kind === "DOCUMENT"
      ? "On hold: renew your documents"
      : "Your account is on hold";

/** One line explaining a hold. `brand` may be unknown; the sentence still reads well. */
export function holdText(h: Hold, w: Wallet | undefined, brand?: string): string {
  if (h.kind === "DOCUMENT") {
    return `Your ${kindsText(h.reason ?? "")} expired. Upload the renewed one under Account, Documents. You can drive again once it is approved.`;
  }
  if (h.kind === "MANUAL") {
    const who = brand ? `${brand} support` : "Our support team";
    const why = h.reason ? `: ${h.reason}` : "";
    return `${who} put your account on hold${why}. Contact support to resolve it.`;
  }
  if (!w) return "You owe the platform. Pay to go online again.";
  return `You owe ${money(debtOf(w), w.currency)}. Pay at least ${money(suggestedPayment(w), w.currency)} to go online again.`;
}

/** The soft-limit warning, when a hold is coming but not here yet. */
export function warningText(w: Wallet): string | undefined {
  if (!w.soft_hold_at || w.holds.length) return undefined;
  const when = new Date(w.soft_hold_at).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
  return `Pay ${money(suggestedPayment(w), w.currency)} by ${when} to avoid a hold.`;
}

/** What the wallet says under the figure: when a hold starts and what payments start from. */
export function limitText(w: Wallet): string {
  return `You are put on hold at ${money(Number(w.limits.hard_limit), w.currency)}, or after ${w.limits.soft_grace_days} days over ${money(Number(w.limits.soft_limit), w.currency)}. Payments start from ${money(Number(w.limits.min_payin), w.currency)}.`;
}

/** What to say after a payment arrived, given the refreshed wallet. */
export function successText(w: Wallet | undefined): string {
  if (!w || !w.holds.length) return "Thank you! You can go online again.";
  if (w.holds.some((h) => h.kind === "MANUAL") && !w.holds.some((h) => h.kind === "DEBT")) {
    return "Thank you! Your balance is updated; a staff hold still needs support to lift it.";
  }
  return `Thank you! Pay ${money(suggestedPayment(w), w.currency)} more to go online.`;
}

const FAILURE_TEXT: Record<string, string> = {
  INSUFFICIENT_FUNDS: "Payment failed: not enough balance.",
  REJECTED_BY_USER: "You cancelled the payment.",
  EXPIRED: "No approval received in time. Try again.",
  GATEWAY_ERROR: "Payment service is unavailable. Try again shortly.",
};

/** Why a payment failed, in plain words. */
export const failureText = (code?: string | null) =>
  (code && FAILURE_TEXT[code]) || "Payment failed. Try again.";

/** Wallet history line labels by ledger kind. */
export const ENTRY_LABEL: Record<string, string> = {
  // Cash trips are a debit (the platform's share), mobile-money trips a credit (your earnings).
  "trip.completed": "Trip",
  "trip.cancellation_fee": "Cancellation fee",
  "trip.dues_collected": "Dues collected",
  "payin.collected": "Payment",
  "payout.paid": "Cash out",
};
