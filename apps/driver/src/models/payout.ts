/**
 * Payout model — what the driver can cash out and where a cash-out stands, in
 * plain words (pure: tested in payout.test.ts).
 */
import type { components } from "@platform/api-client";
import type { Wallet } from "@/models/wallet";

export type Payout = components["schemas"]["Payout"];

export type CashOutState =
  | { kind: "off" }
  | { kind: "open"; payout: Payout }
  | { kind: "nothing" }
  | { kind: "below_min"; available: number; min: number }
  | { kind: "ready"; available: number; min: number };

/** What the cash-out card shows. */
export function cashOutState(w: Wallet): CashOutState {
  const p = w.payouts;
  if (!p?.payouts_enabled) return { kind: "off" };
  if (p.open_payout) return { kind: "open", payout: p.open_payout };
  const available = Math.floor(Number(p.available_payout));
  const min = Number(p.payout_min);
  if (available <= 0) return { kind: "nothing" };
  return available < min ? { kind: "below_min", available, min } : { kind: "ready", available, min };
}

/** One line for a payout's status. */
export function payoutText(p: Payout): string {
  switch (p.status) {
    case "REQUESTED":
      return "Waiting for approval";
    case "PENDING":
      return "Sending to your phone";
    case "PAID":
      return "Paid to your phone";
    case "REJECTED":
      return p.decision_reason ? `Not approved: ${p.decision_reason}` : "Not approved";
    case "CANCELLED":
      return "Cancelled";
    default:
      return FAILURE_TEXT[p.failure_code ?? ""] ?? "Could not be sent. Your balance is unchanged.";
  }
}

const FAILURE_TEXT: Record<string, string> = {
  BALANCE_CHANGED: "Your balance changed before it was sent. Try again.",
  WALLET_INVALID: "Your number has no mobile money wallet. Contact support.",
  WALLET_UNSUPPORTED: "Your network's wallet is not supported yet. Contact support.",
};
