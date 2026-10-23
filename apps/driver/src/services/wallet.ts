/** wallet service — the driver's wallet and mobile-money debt payments. Pure data access over the shared client. */
import type { components } from "@platform/api-client";
import type { Payin, Wallet } from "@/models/wallet";
import { api } from "@/services/api";

type ApiErrorBody = { error?: { code?: string; message?: string } };

/** Error carrying the server's stable code so screens can map it to copy. */
export class WalletError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

function fail(error: unknown, fallback: string): never {
  const e = (error as ApiErrorBody | undefined)?.error;
  throw new WalletError(e?.code ?? "UNKNOWN", e?.message ?? fallback);
}

export async function fetchWallet(signal?: AbortSignal): Promise<Wallet> {
  const { data, error } = await api.GET("/driver/wallet", { signal });
  if (error || !data) fail(error, "Could not read your wallet");
  return data;
}

/** Asks the gateway to prompt the phone. The key makes a retry of the same tap safe. */
export async function createPayin(amount: number, phone: string | undefined, key: string): Promise<Payin> {
  const { data, error } = await api.POST("/driver/payins", {
    params: { header: { "Idempotency-Key": key } },
    body: { amount: String(amount), phone: phone || undefined },
  });
  if (error || !data) fail(error, "Could not start the payment");
  return data;
}

export async function fetchPayin(id: string, signal?: AbortSignal): Promise<Payin> {
  const { data, error } = await api.GET("/driver/payins/{id}", { params: { path: { id } }, signal });
  if (error || !data) fail(error, "Could not read the payment");
  return data;
}

export type Hold = components["schemas"]["Hold"];
