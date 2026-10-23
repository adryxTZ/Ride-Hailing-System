/** Plain names for ledger accounts and transaction kinds shown to finance staff. */
export const ACCOUNT_LABEL: Record<string, string> = {
  DRIVER_WALLET: "Driver wallet",
  PLATFORM_REVENUE: "Platform revenue",
  VAT_PAYABLE: "VAT payable",
  LEVY_PAYABLE: "Levy payable",
  RIDER_DUES: "Rider dues",
  GATEWAY_CLEARING: "Clearing at gateway",
};

/** Why a driver is on hold. */
export const HOLD_CAUSE: Record<string, string> = {
  HARD_LIMIT: "Debt over the hard limit",
  SOFT_LIMIT_OVERDUE: "Debt over the soft limit too long",
  STAFF: "Staff hold",
};

/** Why a payment failed. */
export const FAILURE_LABEL: Record<string, string> = {
  INSUFFICIENT_FUNDS: "Not enough balance",
  REJECTED_BY_USER: "Cancelled by the driver",
  EXPIRED: "No approval in time",
  GATEWAY_ERROR: "Gateway error",
};

/** Why a cash-out was not paid. */
export const PAYOUT_FAILURE: Record<string, string> = {
  BALANCE_CHANGED: "Balance dropped before sending",
  WALLET_INVALID: "No wallet on that number",
  WALLET_UNSUPPORTED: "Wallet not supported",
  FLOAT_TOO_LOW: "Our gateway balance is too low",
  PAYOUT_REFUSED: "Refused by the gateway",
};

/** Turns a code like TRIP_SETTLEMENT or trip.completed into "Trip settlement". */
export function humanize(code: string): string {
  const s = code.toLowerCase().replace(/[_.]/g, " ");
  return s.charAt(0).toUpperCase() + s.slice(1);
}
