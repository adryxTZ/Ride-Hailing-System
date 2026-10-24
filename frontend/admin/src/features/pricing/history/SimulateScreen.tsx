"use client";
/**
 * SimulateScreen — price a trip without booking it: drop a pickup and a
 * drop-off, pick when (within 7 days; a past time uses the profile history)
 * and optionally a draft profile to try. Shows each ride type's fare, its
 * lines, how it would be split, and which profile applied and why.
 */
import { ArrowLeft, Calculator } from "lucide-react";
import Link from "next/link";
import { useCallback, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Select } from "@/components/ui/Field";
import { OperationsHeader } from "@/components/ui/Operations";
import { Empty, ErrorNote } from "@/components/ui/States";
import { Table } from "@/components/ui/Table";
import { useClassName } from "@/lib/catalog";
import { km, money, seconds } from "@/lib/format";
import { useScope } from "@/lib/scope";
import { useProfiles } from "../profiles/hooks";
import { isoOf, type LatLng, localOf, type Simulation, useSimulate } from "./hooks";
import { KIND_LABEL } from "./labels";
import { PointsMap } from "./PointsMap";
import { ResolutionTrace, type Trace } from "./ResolutionTrace";

type Split = {
  commission?: string;
  vat?: string;
  levy?: string;
  driver_earnings?: string;
  platform_net?: string;
  third_party?: string;
};
type Line = { code: string; amount: string; label?: { en?: string } };

const LINE_LABEL: Record<string, string> = {
  BASE_FARE: "Base fare",
  SURGE: "Busy-time increase",
  WAITING: "Waiting",
  PICKUP_APPROACH: "Pickup approach",
  MINIMUM_FARE_ADJUSTMENT: "Minimum fare",
  BOOKING_FEE: "Booking fee",
  FIXED_ROUTE: "Fixed route",
  ROUNDING: "Rounding",
};

export function SimulateScreen() {
  const { scope } = useScope();
  const profiles = useProfiles(scope.country);
  const simulate = useSimulate();
  const [pickup, setPickup] = useState<LatLng>();
  const [dropoff, setDropoff] = useState<LatLng>();
  const [service, setService] = useState("taxi");
  const [at, setAt] = useState(() => localOf(new Date()));
  const [preview, setPreview] = useState("");
  const onPoints = useCallback((a?: LatLng, b?: LatLng) => {
    setPickup(a);
    setDropoff(b);
  }, []);
  const run = () =>
    pickup &&
    simulate.mutate({
      service,
      pickup,
      dropoff,
      at: isoOf(at) || undefined,
      preview_profile_id: preview || undefined,
    });
  const tryable = (profiles.data ?? []).filter((p) => !p.is_default);
  return (
    <div>
      <OperationsHeader
        title="Simulate a trip"
        description="See what a trip would cost, how the fare would be split and which pricing profile applies. Nothing is saved."
        actions={
          <Link
            href="/pricing/fees"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
          >
            <ArrowLeft size={14} aria-hidden="true" /> Pricing profiles
          </Link>
        }
      />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <Card title="Where" index={0}>
          <PointsMap
            pickup={pickup}
            dropoff={dropoff}
            onChange={onPoints}
            polyline={simulate.data?.polyline}
            label="Map: click to place the pickup, then the drop-off"
          />
        </Card>
        <Card title="Trip" index={1}>
          <div className="grid gap-4">
            <Field label="Service">
              <Select value={service} onChange={(e) => setService(e.target.value)}>
                <option value="taxi">Ride</option>
                <option value="parcel">Package</option>
              </Select>
            </Field>
            <Field
              label="When"
              hint="Within 7 days of now. A past time uses the profiles that applied then; tariffs are today's."
            >
              <Input type="datetime-local" value={at} onChange={(e) => setAt(e.target.value)} />
            </Field>
            <Field label="Try a profile" hint="Takes part as if switched on, in this answer only.">
              <Select value={preview} onChange={(e) => setPreview(e.target.value)}>
                <option value="">None, as configured</option>
                {tryable.map((p) => (
                  <option key={p.id} value={p.id}>{`${p.name} (${p.status.toLowerCase()})`}</option>
                ))}
              </Select>
            </Field>
            <Button
              icon={<Calculator size={15} aria-hidden="true" />}
              disabled={!pickup}
              loading={simulate.isPending}
              onClick={run}
            >
              Simulate
            </Button>
            {simulate.error ? <ErrorNote error={simulate.error} /> : null}
          </div>
        </Card>
      </div>
      <div className="mt-4" aria-live="polite">
        {simulate.data ? (
          <Result sim={simulate.data} />
        ) : (
          <Empty title="No simulation yet" body="Place a pickup on the map and press Simulate." />
        )}
      </div>
    </div>
  );
}

function Result({ sim }: { sim: Simulation }) {
  const className = useClassName();
  const res = sim.resolution as {
    profile?: { id: string; name: string; version: number };
    scope?: { kind: string; name: string };
    default?: { name: string; version: number };
    trace?: Trace;
  };
  const splits = sim.splits as Record<string, Split>;
  return (
    <div className="grid gap-4">
      <Card title="Applied profile" index={0}>
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-xs">
          <p>
            <Link
              href={`/pricing/fees/${res.profile?.id}`}
              className="text-sm font-semibold hover:text-brand-text"
            >
              {res.profile?.name}
            </Link>
            <span className="tnum ml-1.5 text-muted">{`v${res.profile?.version}`}</span>
          </p>
          <Chip tone="brand">{`${KIND_LABEL[res.scope?.kind ?? ""] ?? res.scope?.kind}: ${res.scope?.name}`}</Chip>
          <span className="text-muted">{`Default: ${res.default?.name} v${res.default?.version}`}</span>
          <span className="tnum text-muted">{`${km(sim.distance_m)} · ${seconds(sim.duration_s)}`}</span>
        </div>
      </Card>
      <Card title="Fares and splits" index={1}>
        <Table
          head={[
            "Ride type",
            "Fare",
            "Lines",
            "Driver keeps",
            "Platform keeps",
            "Tax and levy",
            "Third parties",
          ]}
        >
          {sim.options.map((o) => {
            const s = splits[o.vehicle_class];
            return (
              <tr key={o.vehicle_class}>
                <td className="px-3 py-3 text-xs font-semibold">{className(o.vehicle_class)}</td>
                <td className="tnum px-3 py-3 text-xs">
                  {o.available ? money(o.fare, o.currency) : "Not priced"}
                </td>
                <td className="px-3 py-3 text-[11px] text-muted">
                  {((o.lines ?? []) as Line[]).map((l) => (
                    <p
                      key={`${l.code}-${l.label?.en ?? ""}`}
                      className="tnum"
                    >{`${l.label?.en ?? LINE_LABEL[l.code] ?? l.code}: ${money(l.amount, o.currency)}`}</p>
                  ))}
                </td>
                <td className="tnum px-3 py-3 text-xs">{s ? money(s.driver_earnings, o.currency) : "—"}</td>
                <td className="tnum px-3 py-3 text-xs">{s ? money(s.platform_net, o.currency) : "—"}</td>
                <td className="tnum px-3 py-3 text-xs">
                  {s ? money(Number(s.vat ?? 0) + Number(s.levy ?? 0), o.currency) : "—"}
                </td>
                <td className="tnum px-3 py-3 text-xs">{s ? money(s.third_party, o.currency) : "—"}</td>
              </tr>
            );
          })}
        </Table>
      </Card>
      <Card title="Why this profile" index={2}>
        <ResolutionTrace trace={res.trace ?? {}} />
      </Card>
    </div>
  );
}
