"use client";
/** TimelineView — which profile applied where, from when to when, who changed it and why. */
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Select } from "@/components/ui/Field";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Table } from "@/components/ui/Table";
import { dateTime, when } from "@/lib/format";
import { STATUS_LABEL, STATUS_TONE } from "../profiles/model";
import { isoOf, localOf, useTimeline } from "./hooks";
import { EVENT_LABEL, KIND_LABEL } from "./labels";

const DEFAULT_DAYS = 90;
const DAY_MS = 86_400_000;
/** Changes shown before "Show all". */
const FIRST_CHANGES = 20;
const KINDS = ["", "COUNTRY", "AREA", "DISTRICT", "ZONE", "PLACE"];

export function TimelineView({ country }: { country: string }) {
  const [from, setFrom] = useState(() => localOf(new Date(Date.now() - DEFAULT_DAYS * DAY_MS)));
  const [to, setTo] = useState(() => localOf(new Date()));
  const [kind, setKind] = useState("");
  const [allChanges, setAllChanges] = useState(false);
  const { data, isLoading, error } = useTimeline(country, isoOf(from), isoOf(to), kind);
  return (
    <div className="grid gap-4">
      <Card index={0}>
        <div className="grid gap-4 sm:grid-cols-3">
          <Field label="From">
            <Input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} />
          </Field>
          <Field label="To">
            <Input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} />
          </Field>
          <Field label="Where">
            <Select value={kind} onChange={(e) => setKind(e.target.value)}>
              {KINDS.map((k) => (
                <option key={k} value={k}>
                  {k ? KIND_LABEL[k] : "Everywhere"}
                </option>
              ))}
            </Select>
          </Field>
        </div>
      </Card>
      {error ? <ErrorNote error={error} /> : null}
      <Card title="Assignments" index={1}>
        {isLoading ? (
          <Skeleton className="h-48" />
        ) : !data?.assignments.length ? (
          <Empty
            title="Nothing in this period"
            body="No profile was applied anywhere in these dates. Widen the range."
          />
        ) : (
          <Table head={["Where", "Profile", "From", "Until", "Applied by", "Why"]}>
            {data.assignments.map((a) => (
              <tr key={a.assignment_id}>
                <td className="px-3 py-3 text-xs">
                  {a.name}
                  <span className="ml-1.5 text-[10px] text-muted">{KIND_LABEL[a.kind] ?? a.kind}</span>
                </td>
                <td className="px-3 py-3 text-xs">
                  <Link
                    href={`/pricing/fees/${a.profile_id}`}
                    className="font-semibold hover:text-brand-text"
                  >
                    {a.profile_name}
                  </Link>
                  <span className="ml-2">
                    <Chip tone={STATUS_TONE[a.profile_status]}>{STATUS_LABEL[a.profile_status]}</Chip>
                  </span>
                </td>
                <td className="tnum whitespace-nowrap px-3 py-3 text-[11px]">{dateTime(a.valid_from)}</td>
                <td className="tnum whitespace-nowrap px-3 py-3 text-[11px]">
                  {a.valid_to ? dateTime(a.valid_to) : <Chip tone="ok">Still applies</Chip>}
                </td>
                <td className="px-3 py-3 text-[11px] text-muted">{a.assigned_by ?? "System"}</td>
                <td className="max-w-64 px-3 py-3 text-[11px] text-muted">
                  {a.reason}
                  {a.close_reason ? (
                    <p className="mt-1">{`Removed: ${a.close_reason}${a.closed_by ? ` (${a.closed_by})` : ""}`}</p>
                  ) : null}
                </td>
              </tr>
            ))}
          </Table>
        )}
      </Card>
      <Card title="Profile changes" index={2}>
        {!data?.events.length ? (
          <Empty title="No changes" body="Profile changes in this period appear here." />
        ) : (
          <ul className="divide-y divide-line">
            {(allChanges ? data.events : data.events.slice(0, FIRST_CHANGES)).map((e, i) => (
              <li
                key={`${e.profile_id}-${e.at}-${i}`}
                className="flex flex-wrap items-baseline justify-between gap-2 py-2.5 text-xs"
              >
                <span>
                  <Link
                    href={`/pricing/fees/${e.profile_id}`}
                    className="font-semibold hover:text-brand-text"
                  >
                    {e.profile_name}
                  </Link>
                  <span className="ml-2">{`${EVENT_LABEL[e.kind] ?? e.kind}${e.version ? ` v${e.version}` : ""}`}</span>
                  {e.reason ? <span className="ml-2 text-muted">{e.reason}</span> : null}
                </span>
                <span className="tnum text-[11px] text-muted">{`${e.actor_name ?? "System"} · ${when(e.at)}`}</span>
              </li>
            ))}
          </ul>
        )}
        {data && data.events.length > FIRST_CHANGES && !allChanges ? (
          <Button variant="soft" size="sm" className="mt-3" onClick={() => setAllChanges(true)}>
            {`Show all ${data.events.length} changes`}
          </Button>
        ) : null}
      </Card>
    </div>
  );
}
