"use client";
/** AsOfView — "what priced this point at that time", rebuilt from the profile history. */
import Link from "next/link";
import { useCallback, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input } from "@/components/ui/Field";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Table } from "@/components/ui/Table";
import { money } from "@/lib/format";
import { isoOf, type LatLng, localOf, useAsOf } from "./hooks";
import { KIND_LABEL, MATCHED_LABEL } from "./labels";
import { PointsMap } from "./PointsMap";
import { ResolutionTrace, type Trace } from "./ResolutionTrace";

const HOUR_MS = 3_600_000;

export function AsOfView({ country }: { country: string }) {
  const [pickup, setPickup] = useState<LatLng>();
  const [dropoff, setDropoff] = useState<LatLng>();
  const [at, setAt] = useState(() => localOf(new Date(Date.now() - HOUR_MS)));
  const onPoints = useCallback((a?: LatLng, b?: LatLng) => {
    setPickup(a);
    setDropoff(b);
  }, []);
  const { data, isLoading, error } = useAsOf(country, isoOf(at), pickup, dropoff);
  return (
    <div className="grid gap-4">
      <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <Card title="Where" index={0}>
          <PointsMap
            pickup={pickup}
            dropoff={dropoff}
            onChange={onPoints}
            label="Map: click to place the point, then an optional drop-off"
          />
        </Card>
        <Card title="When" index={1}>
          <Field
            label="Time"
            hint="Any time in the past. Answered from the history of assignments, versions and status changes."
          >
            <Input
              type="datetime-local"
              value={at}
              max={localOf(new Date())}
              onChange={(e) => setAt(e.target.value)}
            />
          </Field>
        </Card>
      </div>
      <div aria-live="polite">
        {error ? <ErrorNote error={error} /> : null}
        {!pickup ? (
          <Empty title="Pick a point" body="Click the map where the trip started." />
        ) : isLoading || !data ? (
          <Skeleton className="h-48" />
        ) : (
          <div className="grid gap-4">
            {data.approximate ? (
              <p
                role="status"
                className="rounded-[10px] bg-[color-mix(in_srgb,var(--warn)_12%,transparent)] px-4 py-3 text-xs leading-relaxed"
              >
                <strong className="font-semibold">Approximate.</strong> A region here was reshaped after this
                time. Regions are matched with today's shapes; districts, zones and places use the shapes they
                had then.
              </p>
            ) : null}
            <Card title="Applied then" index={0}>
              <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-xs">
                <p>
                  <Link
                    href={`/pricing/fees/${data.profile.id}`}
                    className="text-sm font-semibold hover:text-brand-text"
                  >
                    {data.profile.name}
                  </Link>
                  <span className="tnum ml-1.5 text-muted">{`v${data.profile.version}`}</span>
                </p>
                <Chip tone="brand">{`${KIND_LABEL[data.scope.kind ?? ""] ?? data.scope.kind}: ${data.scope.name}`}</Chip>
                <span className="text-muted">{`Default then: ${data.default_profile.name} v${data.default_profile.version}`}</span>
              </div>
            </Card>
            <Card title="Charges that applied" index={1}>
              {!data.charges.length ? (
                <Empty
                  title="No extra charges"
                  body="No toll, parking, levy or other charge applied here then."
                />
              ) : (
                <Table head={["Charge", "Amount", "Where", "Paid to"]}>
                  {data.charges.map((c) => (
                    <tr key={`${c.code}-${c.matched_at}`}>
                      <td className="px-3 py-3 text-xs font-semibold">{c.label.en}</td>
                      <td className="tnum px-3 py-3 text-xs">
                        {c.amount_type === "PERCENT_OF_FARE"
                          ? `${Number(c.amount) * 100}% of fare`
                          : c.amount_type === "PER_KM"
                            ? `${money(c.amount)} per km`
                            : money(c.amount)}
                      </td>
                      <td className="px-3 py-3 text-xs">
                        {MATCHED_LABEL[c.matched_at ?? ""] ?? c.matched_at}
                      </td>
                      <td className="px-3 py-3 text-xs text-muted">
                        {c.recipient.replace("_", " ").toLowerCase()}
                      </td>
                    </tr>
                  ))}
                </Table>
              )}
            </Card>
            <Card title="Why this profile" index={2}>
              <ResolutionTrace trace={data.resolution as Trace} />
            </Card>
          </div>
        )}
      </div>
    </div>
  );
}
