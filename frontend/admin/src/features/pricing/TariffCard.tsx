"use client";
/** TariffCard — one ride type's prices; edits are saved with a reason and apply to new quotes within seconds. */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Field, Input, Toggle } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { useUpdateTariff } from "./hooks";

const PARAM_LABEL: Record<string, string> = {
  base_fare: "Base fare",
  per_km: "Per km",
  per_min: "Per minute",
  price: "Flat price",
  surge_max: "Surge cap (×)",
};

export function TariffCard({
  t,
  title,
  index,
  editable,
}: {
  t: Schemas["TariffView"];
  title: string;
  index: number;
  editable: boolean;
}) {
  const [draft, setDraft] = useState(() => ({ ...t, params: { ...t.params } }));
  const [asking, setAsking] = useState(false);
  const save = useUpdateTariff();
  const changed = JSON.stringify(draft) !== JSON.stringify({ ...t, params: { ...t.params } });
  const num = (v: string) => v.replace(/[^\d.]/g, "");
  return (
    <Card
      index={index}
      title={title}
      action={<span className="text-xs text-muted">{t.engine.replace(/_/g, " ")}</span>}
    >
      <div className="grid grid-cols-2 gap-3">
        {Object.keys(draft.params)
          .sort()
          .map((k) => (
            <Field key={k} label={PARAM_LABEL[k] ?? k}>
              <Input
                value={draft.params[k]}
                disabled={!editable}
                onChange={(e) =>
                  setDraft({ ...draft, params: { ...draft.params, [k]: num(e.target.value) } })
                }
              />
            </Field>
          ))}
        <Field label="Minimum fare">
          <Input
            value={draft.minimum_fare}
            disabled={!editable}
            onChange={(e) => setDraft({ ...draft, minimum_fare: num(e.target.value) })}
          />
        </Field>
        <Field label="Cancellation fee">
          <Input
            value={draft.cancellation_fee}
            disabled={!editable}
            onChange={(e) => setDraft({ ...draft, cancellation_fee: num(e.target.value) })}
          />
        </Field>
        <Field label="Waiting per minute">
          <Input
            value={draft.waiting_rate_per_min}
            disabled={!editable}
            onChange={(e) => setDraft({ ...draft, waiting_rate_per_min: num(e.target.value) })}
          />
        </Field>
        <Field label="Free waiting (minutes)">
          <Input
            value={String(Math.round(draft.free_waiting_s / 60))}
            disabled={!editable}
            onChange={(e) => setDraft({ ...draft, free_waiting_s: Number(num(e.target.value) || 0) * 60 })}
          />
        </Field>
      </div>
      <div className="mt-4 flex items-center justify-between">
        <label className="flex items-center gap-2 text-sm">
          <Toggle
            label="Surge pricing"
            checked={draft.surge_enabled}
            onChange={(v) => editable && setDraft({ ...draft, surge_enabled: v })}
          />
          Surge pricing
        </label>
        {editable ? (
          <Button size="sm" disabled={!changed} onClick={() => setAsking(true)}>
            Save prices
          </Button>
        ) : null}
      </div>
      <ErrorNote error={save.error} />
      <ReasonDialog
        open={asking}
        title={`Change ${title} prices?`}
        body="New quotes use these prices within seconds. Trips already booked keep their price."
        confirm="Save"
        busy={save.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) =>
          save.mutate(
            {
              service: t.service,
              cls: t.vehicle_class,
              body: {
                engine: draft.engine,
                params: draft.params,
                minimum_fare: draft.minimum_fare,
                waiting_rate_per_min: draft.waiting_rate_per_min,
                free_waiting_s: draft.free_waiting_s,
                cancellation_fee: draft.cancellation_fee,
                surge_enabled: draft.surge_enabled,
                reason,
              },
            },
            { onSuccess: () => setAsking(false) },
          )
        }
      />
    </Card>
  );
}
