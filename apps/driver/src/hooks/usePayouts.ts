/**
 * usePayouts — the driver's cash-outs: recent ones, asking for one and
 * withdrawing a request. The wallet refreshes after each change (the server
 * also pushes wallet.updated when Finance or the gateway moves a payout).
 */
import { useSession } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef } from "react";
import { cancelPayout, createPayout, fetchPayouts } from "@/services/payouts";

const KEY = ["driver", "payouts"];

export function usePayouts() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: KEY,
    queryFn: ({ signal }) => fetchPayouts(signal),
    enabled: isSignedIn,
    staleTime: 15_000,
  });
}

export function usePayoutActions() {
  const qc = useQueryClient();
  // One key per attempt: a network retry of the same tap never asks twice.
  const key = useRef(newKey());
  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ["driver", "wallet"] });
    void qc.invalidateQueries({ queryKey: KEY });
  };
  const request = useMutation({
    mutationFn: (amount: number) => createPayout(amount, key.current),
    onSuccess: () => {
      key.current = newKey();
      refresh();
    },
  });
  const cancel = useMutation({ mutationFn: (id: string) => cancelPayout(id), onSuccess: refresh });
  return { request, cancel };
}

function newKey(): string {
  return `payout-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}
