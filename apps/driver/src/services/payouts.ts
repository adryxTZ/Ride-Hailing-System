/** payouts service — cash-outs to the account phone. Pure data access over the shared client. */
import type { Payout } from "@/models/payout";
import { api } from "@/services/api";
import { WalletError } from "@/services/wallet";

function fail(error: unknown, fallback: string): never {
  const e = (error as { error?: { code?: string; message?: string } } | undefined)?.error;
  throw new WalletError(e?.code ?? "UNKNOWN", e?.message ?? fallback);
}

/** Asks to cash out. The key makes a retry of the same tap safe. */
export async function createPayout(amount: number, key: string): Promise<Payout> {
  const { data, error } = await api.POST("/driver/payouts", {
    params: { header: { "Idempotency-Key": key } },
    body: { amount: String(amount) },
  });
  if (error || !data) fail(error, "Could not request the cash-out");
  return data;
}

export async function cancelPayout(id: string): Promise<Payout> {
  const { data, error } = await api.POST("/driver/payouts/{id}/cancel", { params: { path: { id } } });
  if (error || !data) fail(error, "Could not cancel");
  return data;
}

export async function fetchPayouts(signal?: AbortSignal): Promise<Payout[]> {
  const { data, error } = await api.GET("/driver/payouts", { signal });
  if (error || !data) fail(error, "Could not read your cash-outs");
  return data.data;
}
