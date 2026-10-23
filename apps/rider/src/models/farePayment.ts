/**
 * farePayment — what the rider's end-of-ride card shows, derived from the trip's
 * payment (pure: no React, tested in farePayment.test.ts).
 */
import type { Trip } from "@/services/trips";

export type FareView =
  | { kind: "approve"; phone: string; secondsLeft: number }
  | { kind: "failed"; reason: string; promptsLeft: number }
  | { kind: "cash"; canUseMobileMoney: boolean }
  | { kind: "paid" };

/** Plain words for each gateway failure. */
const FAILURE_TEXT: Record<string, string> = {
  INSUFFICIENT_FUNDS: "There was not enough money on that number.",
  REJECTED_BY_USER: "The payment was declined on the phone.",
  EXPIRED: "The payment request timed out.",
  GATEWAY_ERROR: "Mobile money is not responding right now.",
};

export function fareView(trip: Trip, now: number): FareView {
  const pay = trip.payment;
  if (!pay || pay.status === "COLLECTED" || pay.status === "CASH_CONFIRMED") return { kind: "paid" };
  const prompt = pay.prompt;
  if (pay.status === "PENDING" && prompt) {
    const secondsLeft = Math.max(0, Math.round((Date.parse(prompt.expires_at) - now) / 1000));
    return { kind: "approve", phone: prompt.phone, secondsLeft };
  }
  const promptsLeft = prompt ? prompt.prompts_left : 3;
  if (pay.status === "FAILED" && !pay.cash_chosen) {
    const reason = FAILURE_TEXT[prompt?.failure_code ?? ""] ?? "The payment did not go through.";
    return { kind: "failed", reason, promptsLeft };
  }
  return { kind: "cash", canUseMobileMoney: !pay.cash_chosen && promptsLeft > 0 };
}
