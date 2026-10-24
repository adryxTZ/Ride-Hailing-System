"use client";

import {
  ArrowUpRight,
  Banknote,
  CalendarDays,
  CarFront,
  CheckCheck,
  ChevronRight,
  Clock3,
  Flame,
  LockKeyhole,
  Route,
  ScanSearch,
  ShieldAlert,
  UserCheck,
} from "lucide-react";
import Link from "next/link";
import { Chart } from "@/components/charts/Chart";
import { Chip, type Tone } from "@/components/ui/Chip";
import {
  MetricTile,
  OperationsHeader,
  OperationsPanel,
  RefreshButton,
  UpdateIndicator,
} from "@/components/ui/Operations";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { cents, count, money, percent } from "@/lib/format";
import { useCountries, useRegulator, useScope } from "@/lib/scope";
import { useOverview } from "./hooks";

const hourLabel = (iso: string) =>
  new Date(iso).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });

export function OverviewScreen() {
  const regulator = useRegulator();
  const overview = useOverview();
  const { scope } = useScope();
  const { data: countries } = useCountries();
  const { data: o, isLoading, error, dataUpdatedAt, isFetching } = overview;
  const t = o?.trips;
  const cur =
    o?.finance?.currency ?? countries?.find((country) => country.code === scope.country)?.currency ?? "TZS";
  const review = o?.driver_statuses?.PENDING_REVIEW ?? 0;
  const attention = (o?.open_sos ?? 0) + review + (o?.fraud_open ?? 0) + (o?.privacy_overdue ?? 0);
  return (
    <div>
      <OperationsHeader
        title="Overview"
        description="Your operation today. Track demand, driver availability, and the work that needs attention."
        actions={
          <>
            <UpdateIndicator at={dataUpdatedAt} error={error} fetching={isFetching} interval="30s" />
            <Link
              href="/live"
              className="inline-flex h-9 items-center gap-2 rounded-lg bg-nav px-3 text-xs font-semibold text-nav-ink"
            >
              Open live map <ArrowUpRight size={14} aria-hidden="true" />
            </Link>
          </>
        }
      />
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2 text-[11px] text-muted">
        <span className="inline-flex items-center gap-1.5">
          <CalendarDays size={13} aria-hidden="true" />
          {t
            ? `Today · ${new Date(t.since).toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" })}`
            : "Today"}
        </span>
        <span>Since midnight in the selected country</span>
      </div>
      {error ? (
        <div className="mb-4 flex flex-wrap items-start gap-3">
          <div className="min-w-0 flex-1">
            <ErrorNote error={error} />
          </div>
          <RefreshButton onClick={() => void overview.refetch()} busy={isFetching} />
        </div>
      ) : null}
      {isLoading ? (
        <div role="status" aria-label="Loading overview" className="grid gap-5">
          <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-40" />
            ))}
          </div>
          <Skeleton className="h-96" />
        </div>
      ) : !o || !t ? (
        <Empty title="Overview unavailable" body="Use Refresh to load your operation’s latest figures." />
      ) : (
        <div className="grid gap-5">
          <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
            <MetricTile
              label="Trips requested"
              value={count(t.requested)}
              numericValue={t.requested}
              previous={t.previous.requested}
              note={`${count(t.active)} trips in progress right now`}
              icon={<Route size={17} />}
              accent
            />
            <MetricTile
              label="Trips completed"
              value={count(t.completed)}
              numericValue={t.completed}
              previous={t.previous.completed}
              note={`${percent(t.completion_rate)} completion rate on finished trips`}
              icon={<CheckCheck size={17} />}
            />
            <MetricTile
              label="Gross fares"
              value={money(t.gross_fares, cur)}
              numericValue={Number(t.gross_fares)}
              previous={Number(t.previous.gross_fares)}
              note={`Average fare ${money(t.avg_fare, cur)}`}
              icon={<Banknote size={17} />}
            />
            <MetricTile
              label="Drivers online"
              value={count(o.drivers_online)}
              note={`${count(o.drivers_on_trip)} currently on a trip`}
              icon={<CarFront size={17} />}
            />
          </div>
          <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1.8fr)_minmax(300px,1fr)]">
            <OperationsPanel
              title="Trip activity"
              description="Requests and completions by hour"
              action={
                <Link
                  href="/trips"
                  className="inline-flex items-center gap-1 text-xs font-medium text-brand-text"
                >
                  View trips <ArrowUpRight size={13} aria-hidden="true" />
                </Link>
              }
            >
              <div className="grid grid-cols-3 divide-x divide-line border-b border-line bg-surface-2/40 px-2 py-4">
                <ChartFact label="Requested" value={count(t.requested)} />
                <ChartFact label="Completed" value={count(t.completed)} />
                <ChartFact label="Completion rate" value={percent(t.completion_rate)} />
              </div>
              <div className="p-3 sm:p-5">
                {!t.by_hour.length ? (
                  <Empty
                    title="Your day starts here"
                    body="Trip activity will appear as riders make requests."
                    icon={<Route size={22} />}
                  />
                ) : (
                  <Chart
                    type="area"
                    height={280}
                    series={[
                      {
                        name: "Requested",
                        data: t.by_hour.map((h) => ({ x: new Date(h.hour).getTime(), y: h.requested })),
                      },
                      {
                        name: "Completed",
                        data: t.by_hour.map((h) => ({ x: new Date(h.hour).getTime(), y: h.completed })),
                      },
                    ]}
                    options={{
                      stroke: { curve: "straight", width: 2.5 },
                      fill: { type: "gradient", gradient: { opacityFrom: 0.22, opacityTo: 0.01 } },
                      xaxis: { type: "datetime", labels: { datetimeUTC: false, format: "HH:mm" } },
                      yaxis: {
                        min: 0,
                        forceNiceScale: true,
                        labels: { formatter: (value) => Math.round(value).toLocaleString("en-US") },
                      },
                      tooltip: { x: { format: "dd MMM, HH:mm" } },
                    }}
                    table={{
                      head: ["Hour", "Requested", "Completed"],
                      rows: t.by_hour.map((h) => [hourLabel(h.hour), h.requested, h.completed]),
                    }}
                  />
                )}
              </div>
              <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-5 py-3 text-[11px] text-muted">
                <span>Figures refresh automatically every 30 seconds</span>
                <span className="tnum">{`${count(t.cancelled)} cancelled · ${count(t.no_driver)} unmatched`}</span>
              </div>
            </OperationsPanel>
            <OperationsPanel
              title="Operations queue"
              description="A direct path to your next action"
              action={
                <Chip tone={attention ? "warn" : "ok"}>
                  {attention ? `${count(attention)} to review` : "No reviews waiting"}
                </Chip>
              }
            >
              <ul className="divide-y divide-line px-2">
                <Attention
                  href="/safety"
                  icon={<ShieldAlert size={17} />}
                  label="Safety alerts"
                  description="Unresolved requests for help"
                  value={o.open_sos}
                  tone={o.open_sos ? "bad" : "muted"}
                />
                <Attention
                  href="/compliance/fraud"
                  icon={<ScanSearch size={17} />}
                  label="Fraud flags"
                  description="Possible fake GPS, self-booking or cancel abuse"
                  value={o.fraud_open ?? 0}
                  tone={o.fraud_open ? "warn" : "muted"}
                />
                <Attention
                  href="/compliance/privacy"
                  icon={<LockKeyhole size={17} />}
                  label="Privacy requests due"
                  description="Data copies or deletions past their due date"
                  value={o.privacy_overdue ?? 0}
                  tone={o.privacy_overdue ? "bad" : "muted"}
                />
                <Attention
                  href="/drivers?status=PENDING_REVIEW"
                  icon={<UserCheck size={17} />}
                  label="Driver applications"
                  description="Submitted and ready for review"
                  value={review}
                  tone={review ? "warn" : "muted"}
                />
                <Attention
                  href="/trips?status=active"
                  icon={<Route size={17} />}
                  label="Trips in progress"
                  description="Follow active rides and deliveries"
                  value={t.active}
                  tone="live"
                />
                <Attention
                  href="/trips?status=scheduled"
                  icon={<Clock3 size={17} />}
                  label="Scheduled trips"
                  description="Rides booked ahead of time"
                  value={t.scheduled}
                  tone="muted"
                />
                <Attention
                  href="/live"
                  icon={<Flame size={17} />}
                  label="Busy areas"
                  description="Areas with a surge multiplier"
                  value={o.surge_areas}
                  tone={o.surge_areas ? "warn" : "muted"}
                />
                <Attention
                  href="/trips?status=cancelled"
                  icon={<CarFront size={17} />}
                  label="Unmatched today"
                  description="Requests that found no driver"
                  value={t.no_driver}
                  tone={t.no_driver ? "bad" : "muted"}
                />
              </ul>
            </OperationsPanel>
          </div>
          {o.finance ? (
            <OperationsPanel
              title="Money position"
              description="Current balances, revenue and amounts payable"
              action={
                <Link
                  href="/finance"
                  className="inline-flex items-center gap-1 text-xs font-medium text-brand-text"
                >
                  Open finance <ArrowUpRight size={13} aria-hidden="true" />
                </Link>
              }
            >
              <dl className="grid grid-cols-1 divide-line sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
                <Figure label="Platform revenue" value={money(o.finance.platform_revenue, cur)} accent />
                <Figure label="Drivers owe · cash" value={money(o.finance.drivers_owe, cur)} />
                <Figure label="Owed to drivers" value={money(o.finance.owed_to_drivers, cur)} />
                <Figure label="VAT payable" value={`${cur} ${cents(o.finance.vat_payable)}`} />
                <Figure
                  label={`${regulator.levy} payable`}
                  value={`${cur} ${cents(o.finance.levy_payable)}`}
                />
                <Figure label="Rider cancellation dues" value={money(o.finance.rider_dues, cur)} />
              </dl>
            </OperationsPanel>
          ) : null}
        </div>
      )}
    </div>
  );
}

function Attention({
  href,
  icon,
  label,
  description,
  value,
  tone,
}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  description: string;
  value: number;
  tone: Tone;
}) {
  return (
    <li>
      <Link
        href={href}
        className="group flex items-center gap-3 rounded-lg px-3 py-4 transition-colors hover:bg-surface-2"
      >
        <span
          aria-hidden="true"
          className={`grid size-9 shrink-0 place-items-center rounded-lg ${value && tone === "bad" ? "bg-critical/10 text-critical" : value && tone === "warn" ? "bg-warn/10 text-warn" : "bg-surface-2 text-muted"}`}
        >
          {icon}
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-xs font-medium">{label}</p>
          <p className="mt-1 text-[10px] leading-relaxed text-muted">{description}</p>
        </div>
        <Chip tone={tone}>{count(value)}</Chip>
        <ChevronRight
          size={13}
          className="shrink-0 text-muted transition-transform group-hover:translate-x-0.5"
          aria-hidden="true"
        />
      </Link>
    </li>
  );
}

function ChartFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 px-3">
      <p className="text-[10px] text-muted">{label}</p>
      <p className="tnum mt-1 text-lg font-semibold">{value}</p>
    </div>
  );
}

function Figure({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div
      className={`min-w-0 border-b border-line px-5 py-5 xl:border-b-0 xl:not-last:border-r ${accent ? "bg-brand-soft/50" : ""}`}
    >
      <dt className="text-[11px] leading-relaxed text-muted">{label}</dt>
      <dd className={`tnum mt-2 break-words text-base font-semibold ${accent ? "text-brand-text" : ""}`}>
        {value}
      </dd>
    </div>
  );
}
