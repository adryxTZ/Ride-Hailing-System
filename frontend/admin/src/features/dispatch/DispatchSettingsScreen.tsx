"use client";
/**
 * DispatchSettingsScreen — how drivers are matched, per country. Today one
 * setting: offer the next trip to a driver who is finishing one (trip queue),
 * and which trip statuses count as finishing. Every change asks for a reason
 * and is audited.
 */
import { ListChecks } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import { when } from "@/lib/format";
import { useCountries } from "@/lib/scope";
import {
  type DispatchChange,
  type DispatchSettings,
  type FinishingStatus,
  useDispatchSettings,
  useSetDispatchSettings,
} from "./hooks";

/** The statuses an operator may count as finishing, in trip order, with their labels. */
const FINISHING: { status: FinishingStatus; label: string }[] = [
  { status: "IN_TRANSIT", label: "On the way to the drop-off" },
  { status: "ARRIVED_AT_DROPOFF", label: "At the drop-off" },
  { status: "AWAITING_PAYMENT", label: "Collecting payment" },
];

export function DispatchSettingsScreen() {
  const { data, isLoading, error } = useDispatchSettings();
  const save = useSetDispatchSettings();
  const editable = useMe().can("dispatch.manage");
  const [ask, setAsk] = useState<Omit<DispatchChange, "reason">>();
  return (
    <div>
      <PageHeader
        title="Dispatch"
        subtitle="How drivers are matched to ride requests in each country. Changes reach every dispatcher within seconds."
      />
      <ErrorNote error={error ?? save.error} />
      {isLoading ? (
        <Skeleton className="h-48" />
      ) : !data?.length ? (
        <Empty title="No countries" body="Countries in your scope appear here." />
      ) : (
        <div className={`grid gap-5 ${data.length > 1 ? "xl:grid-cols-2" : ""}`}>
          {data.map((s, i) => (
            <CountryCard key={s.country} index={i + 1} settings={s} editable={editable} onChange={setAsk} />
          ))}
        </div>
      )}
      <ReasonDialog
        open={!!ask}
        title={ask ? changeTitle(ask) : ""}
        body="Recorded in the audit log."
        confirm="Save"
        busy={save.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) => ask && save.mutate({ ...ask, reason }, { onSuccess: () => setAsk(undefined) })}
      />
    </div>
  );
}

function changeTitle(c: Omit<DispatchChange, "reason">): string {
  if (c.trip_queue_enabled !== undefined)
    return `Turn ${c.trip_queue_enabled ? "on" : "off"} offering the next trip in ${c.country}?`;
  return `Change what counts as finishing in ${c.country}?`;
}

function CountryCard({
  settings: s,
  index,
  editable,
  onChange,
}: {
  settings: DispatchSettings;
  index: number;
  editable: boolean;
  onChange: (c: Omit<DispatchChange, "reason">) => void;
}) {
  const { data: countries } = useCountries();
  const name = countries?.find((c) => c.code === s.country)?.name?.en ?? s.country;
  const [draft, setDraft] = useState<FinishingStatus[]>();
  const statuses = draft ?? s.finishing_statuses;
  const dirty = !!draft && !sameSet(draft, s.finishing_statuses);
  const toggle = (st: FinishingStatus, on: boolean) =>
    setDraft(FINISHING.map((f) => f.status).filter((x) => (x === st ? on : statuses.includes(x))));
  return (
    <Card
      index={index}
      title={name}
      action={
        <Chip tone={s.trip_queue_enabled ? "ok" : "muted"}>
          {s.trip_queue_enabled ? "Next trip on" : "Next trip off"}
        </Chip>
      }
    >
      <label
        className={`flex items-center justify-between gap-3 rounded-[8px] bg-surface-2 px-3 py-2 text-sm ${editable ? "cursor-pointer" : "opacity-60"}`}
      >
        <span>
          <span className="block font-medium">Offer the next trip while a driver is finishing</span>
          <span className="block text-xs text-muted">
            Drivers close to a new request can accept it before their current trip ends. Idle drivers about as
            close are preferred.
          </span>
        </span>
        <input
          type="checkbox"
          role="switch"
          className="size-4 shrink-0 accent-[var(--brand)]"
          checked={s.trip_queue_enabled}
          disabled={!editable}
          onChange={(e) => onChange({ country: s.country, trip_queue_enabled: e.target.checked })}
        />
      </label>
      <p className="mt-4 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted">
        <ListChecks size={12} aria-hidden="true" />A driver is finishing when the trip is
      </p>
      <div className="mt-2 grid gap-1.5">
        {FINISHING.map((f) => (
          <label
            key={f.status}
            className={`flex items-center gap-2 text-sm ${editable ? "cursor-pointer" : "opacity-60"}`}
          >
            <input
              type="checkbox"
              className="size-4 accent-[var(--brand)]"
              checked={statuses.includes(f.status)}
              disabled={!editable || (statuses.length === 1 && statuses.includes(f.status))}
              onChange={(e) => toggle(f.status, e.target.checked)}
            />
            {f.label}
          </label>
        ))}
      </div>
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs text-muted">
          {s.updated_at ? `Changed ${when(s.updated_at)}` : "Default settings"}
        </p>
        {dirty ? (
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={() => setDraft(undefined)}>
              Undo
            </Button>
            <Button
              size="sm"
              onClick={() => {
                onChange({ country: s.country, finishing_statuses: statuses });
                setDraft(undefined);
              }}
            >
              Save statuses
            </Button>
          </div>
        ) : null}
      </div>
    </Card>
  );
}

function sameSet(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((x) => b.includes(x));
}
