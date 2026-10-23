"use client";
/** TripPaymentPanel — how the fare was settled and the trip's receipt (number, PDF in English or Swahili). */
import { FileDown, ReceiptText } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Chip, type Tone } from "@/components/ui/Chip";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { useOpenReceipt, useTripReceipt } from "./hooks";

const PAYMENT: Record<string, { label: string; tone: Tone }> = {
  DUE: { label: "Fare due", tone: "warn" },
  PENDING: { label: "Rider approving on phone", tone: "live" },
  FAILED: { label: "Mobile money failed", tone: "bad" },
  COLLECTED: { label: "Paid by mobile money", tone: "ok" },
  CASH_CONFIRMED: { label: "Paid in cash", tone: "ok" },
};

export function TripPaymentPanel({ trip }: { trip: Schemas["Trip"] }) {
  const completed = trip.status === "COMPLETED";
  const { data: receipt, isLoading, error } = useTripReceipt(trip.id, completed);
  const open = useOpenReceipt(trip.id);
  const pay = trip.payment_status ? PAYMENT[trip.payment_status] : undefined;
  if (!pay && !receipt && !completed) return null;
  return (
    <section aria-label="Payment and receipt" className="grid gap-3 border-t border-line px-5 py-4 text-sm">
      <h3 className="flex items-center gap-2 text-xs font-semibold">
        <ReceiptText size={15} className="text-muted" aria-hidden="true" />
        Payment & receipt
      </h3>
      <div className="flex flex-wrap items-center gap-2">
        {pay ? <Chip tone={pay.tone}>{pay.label}</Chip> : null}
        {trip.payment?.cash_chosen && trip.payment_status !== "CASH_CONFIRMED" ? (
          <Chip tone="muted">Rider chose cash</Chip>
        ) : null}
        {trip.payment?.prompt ? (
          <span className="tnum text-xs text-muted">{`${trip.payment.prompt.prompts_used} of 3 prompts used`}</span>
        ) : null}
      </div>
      {isLoading ? <Skeleton className="h-20" /> : null}
      <ErrorNote error={error ?? open.error} />
      {receipt ? (
        <div className="rounded-lg border border-line p-3">
          <div className="flex items-center gap-2">
            <ReceiptText size={17} className="shrink-0 text-muted" aria-hidden="true" />
            <div className="min-w-0">
              <p className="text-[10px] text-muted">Receipt number</p>
              <p className="mt-0.5 break-all font-mono text-xs font-medium">{receipt.number}</p>
            </div>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="primary"
              icon={<FileDown size={14} aria-hidden="true" />}
              loading={open.isPending && open.variables === "en"}
              disabled={open.isPending}
              onClick={() => open.mutate("en")}
            >
              Download PDF
            </Button>
            <Button
              size="sm"
              variant="soft"
              loading={open.isPending && open.variables === "sw"}
              disabled={open.isPending}
              onClick={() => open.mutate("sw")}
            >
              Kiswahili
            </Button>
          </div>
        </div>
      ) : null}
    </section>
  );
}
