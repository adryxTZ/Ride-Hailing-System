/**
 * usePayin — one "Pay now" attempt: idle → submitting → pending → success | failed.
 * A payment already waiting on the phone (the wallet's pending_payin) is adopted,
 * so closing and reopening the sheet never loses it. The outcome arrives over the
 * socket (wallet.updated refreshes the wallet query, which this hook watches);
 * polling every POLL_MS is the fallback, with one last check after expiry.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { failureText, type Payin, type PayinPhase } from "@/models/wallet";
import { createPayin, fetchPayin, WalletError } from "@/services/wallet";

const POLL_MS = 5_000;
/** How long after expires_at to make the final check before giving up. */
const EXPIRY_GRACE_MS = 10_000;
export const WALLET_KEY = ["driver", "wallet"] as const;

/** A client-chosen idempotency key (8–64 chars): time plus randomness is enough to never repeat on one phone. */
const newKey = () => `p-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;

export function usePayin(pendingFromWallet?: Payin | null) {
  const qc = useQueryClient();
  const [phase, setPhase] = useState<PayinPhase>("idle");
  const [payin, setPayin] = useState<Payin>();
  const [error, setError] = useState<string>();
  // One key per attempt: a retry of the same tap replays, a new tap starts afresh.
  const key = useRef(newKey());
  // A payment this hook already settled (or gave up on): the wallet query may still
  // list it as pending for a moment, and it must not be adopted again.
  const settledId = useRef<string | undefined>(undefined);

  const settle = useCallback(
    (p: Payin) => {
      setPayin(p);
      if (p.status !== "PENDING") settledId.current = p.id;
      if (p.status === "COLLECTED") setPhase("success");
      else if (p.status === "FAILED") {
        setPhase("failed");
        setError(failureText(p.failure_code));
      } else setPhase("pending");
      if (p.status !== "PENDING") void qc.invalidateQueries({ queryKey: WALLET_KEY });
    },
    [qc],
  );

  // Adopt a payment already waiting for the PIN (app reopened, sheet closed and reopened).
  useEffect(() => {
    if (phase === "idle" && pendingFromWallet && pendingFromWallet.id !== settledId.current)
      settle(pendingFromWallet);
  }, [phase, pendingFromWallet, settle]);

  // The wallet refreshed (socket frame or refetch) and no longer shows our payment as pending: read its outcome.
  const payinId = payin?.id;
  useEffect(() => {
    if (phase === "pending" && payinId && pendingFromWallet === null) {
      fetchPayin(payinId)
        .then(settle)
        .catch(() => undefined);
    }
  }, [phase, payinId, pendingFromWallet, settle]);

  const start = useCallback(async (amount: number, phone?: string) => {
    setPhase("submitting");
    setError(undefined);
    try {
      const p = await createPayin(amount, phone, key.current);
      setPayin(p);
      setPhase(p.status === "PENDING" ? "pending" : p.status === "COLLECTED" ? "success" : "failed");
      if (p.status === "FAILED") setError(failureText(p.failure_code));
    } catch (e) {
      setPhase("failed");
      setError(e instanceof WalletError ? e.message : "Could not start the payment");
    }
  }, []);

  /** Back to the form. Never called while a payment is pending: that payment keeps waiting. */
  const reset = useCallback(() => {
    key.current = newKey();
    setPhase("idle");
    setPayin(undefined);
    setError(undefined);
  }, []);

  // Poll while pending (the socket usually wins); one final check after expiry, then give up.
  const expiresAt = payin?.expires_at;
  useEffect(() => {
    if (phase !== "pending" || !payinId || !expiresAt) return;
    const deadline = new Date(expiresAt).getTime() + EXPIRY_GRACE_MS;
    const id = setInterval(() => {
      const last = Date.now() >= deadline;
      if (last) clearInterval(id);
      fetchPayin(payinId)
        .then((p) => {
          if (last && p.status === "PENDING") settle({ ...p, status: "FAILED", failure_code: "EXPIRED" });
          else settle(p);
        })
        .catch(() => {
          // The final check could not be made: the prompt is long gone either way, so show
          // the expiry rather than leave the sheet waiting on a network blip.
          if (last && payin) settle({ ...payin, status: "FAILED", failure_code: "EXPIRED" });
        });
    }, POLL_MS);
    return () => clearInterval(id);
  }, [phase, payinId, expiresAt, settle, payin]);

  return { phase, payin, error, start, reset, settle };
}
