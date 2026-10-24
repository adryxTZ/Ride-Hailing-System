"use client";
/**
 * TripFareActions — staff fare actions on a trip: set the price by hand before
 * it is paid, and refund part or all of it after completion (who bears the
 * refund: the platform, or recovered from the driver). Both ask for a reason.
 */
import { BadgeDollarSign, Undo2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { dateTime, money } from "@/lib/format";
import { useRefund, useSetPrice, useTripRefunds } from "./hooks";

const ENDED = [
  "COMPLETED",
  "CANCELLED_BY_RIDER",
  "CANCELLED_BY_DRIVER",
  "CANCELLED_BY_STAFF",
  "NO_DRIVER_FOUND",
  "EXPIRED",
];
const PAID = ["PENDING", "COLLECTED", "CASH_CONFIRMED"];
const BEARER: Record<string, string> = { PLATFORM: "Platform", DRIVER: "Driver" };

export function TripFareActions({ trip }: { trip: Schemas["Trip"] }) {
  const me = useMe();
  const completed = trip.status === "COMPLETED";
  const canPrice =
    me.can("trips.price") && !ENDED.includes(trip.status) && !PAID.includes(trip.payment_status ?? "");
  const refunds = useTripRefunds(trip.id, completed);
  if (!canPrice && !completed) return null;
  return (
    <section aria-label="Fare actions" className="grid gap-3 border-t border-line px-5 py-4 text-sm">
      <h3 className="flex items-center gap-2 text-xs font-semibold">
        <BadgeDollarSign size={15} className="text-muted" aria-hidden="true" />
        Fare actions
      </h3>
      {canPrice ? <SetPrice trip={trip} /> : null}
      {completed && refunds.data ? (
        <Refunds trip={trip} data={refunds.data} canRefund={me.can("trips.refund")} />
      ) : null}
      <ErrorNote error={refunds.error} />
    </section>
  );
}

function SetPrice({ trip }: { trip: Schemas["Trip"] }) {
  const set = useSetPrice();
  const [open, setOpen] = useState(false);
  const [amount, setAmount] = useState("");
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <p className="text-xs text-muted">
        Agreed a different price with the rider? Set it before the fare is paid.
      </p>
      <Button variant="soft" size="sm" onClick={() => setOpen(true)}>
        Set price
      </Button>
      <ErrorNote error={set.error} />
      <ReasonDialog
        open={open}
        title="Set this trip's price"
        body={`Replaces the fare (now ${money(trip.final_fare ?? trip.estimated_fare, trip.currency)}). The rider and driver see the new price; the receipt shows it as set by support.`}
        confirm="Set price"
        busy={set.isPending}
        onClose={() => setOpen(false)}
        onConfirm={(reason) =>
          Number(amount) > 0 &&
          set.mutate({ id: trip.id, amount, reason }, { onSuccess: () => setOpen(false) })
        }
      >
        <Field label={`New fare (${trip.currency})`}>
          <Input
            inputMode="decimal"
            value={amount}
            onChange={(e) => setAmount(e.target.value.replace(/[^0-9.]/g, ""))}
          />
        </Field>
      </ReasonDialog>
    </div>
  );
}

function Refunds({
  trip,
  data,
  canRefund,
}: {
  trip: Schemas["Trip"];
  data: Schemas["TripRefunds"];
  canRefund: boolean;
}) {
  const refund = useRefund();
  const [open, setOpen] = useState(false);
  const [amount, setAmount] = useState("");
  const [bearer, setBearer] = useState<"PLATFORM" | "DRIVER">("PLATFORM");
  const [key, setKey] = useState(""); // one per dialog opening: a double click cannot refund twice
  const left = Number(data.refundable);
  return (
    <div className="grid gap-2">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="tnum text-xs text-muted">{`Paid ${money(data.paid, data.currency)} · refunded ${money(data.refunded, data.currency)} · ${money(data.refundable, data.currency)} can be refunded`}</p>
        {canRefund && left > 0 ? (
          <Button
            variant="soft"
            size="sm"
            icon={<Undo2 size={14} aria-hidden="true" />}
            onClick={() => {
              setKey(crypto.randomUUID());
              setOpen(true);
            }}
          >
            Refund
          </Button>
        ) : null}
      </div>
      {data.refunds.map((r) => (
        <div
          key={r.id}
          className="flex flex-wrap items-center justify-between gap-2 rounded-[8px] bg-surface-2 px-3 py-2 text-xs"
        >
          <span className="tnum font-semibold">{money(r.amount, r.currency)}</span>
          <span className="text-muted">{`${BEARER[r.borne_by]} bears it · ${r.reason} · ${r.created_by ?? "Staff"}, ${dateTime(r.created_at)}`}</span>
          <Chip tone={r.status === "PAID" ? "ok" : "warn"}>
            {r.status === "PAID" ? "Paid to rider" : "Owed to rider"}
          </Chip>
        </div>
      ))}
      <ErrorNote error={refund.error} />
      <ReasonDialog
        open={open}
        title="Refund this trip"
        body="The refund is owed to the rider until Finance pays it out."
        confirm="Refund"
        busy={refund.isPending}
        onClose={() => setOpen(false)}
        onConfirm={(reason) =>
          Number(amount) > 0 &&
          refund.mutate(
            { id: trip.id, key, amount, borne_by: bearer, reason },
            { onSuccess: () => setOpen(false) },
          )
        }
      >
        <Field label={`Amount (${data.currency})`} hint={`Up to ${money(data.refundable, data.currency)}.`}>
          <Input
            inputMode="decimal"
            value={amount}
            onChange={(e) => setAmount(e.target.value.replace(/[^0-9.]/g, ""))}
          />
        </Field>
        <Field label="Who bears it">
          <Select value={bearer} onChange={(e) => setBearer(e.target.value as "PLATFORM" | "DRIVER")}>
            <option value="PLATFORM">The platform</option>
            {trip.driver_id ? <option value="DRIVER">The driver (taken from their earnings)</option> : null}
          </Select>
        </Field>
      </ReasonDialog>
    </div>
  );
}
