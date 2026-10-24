"use client";

import {
  ArrowLeft,
  ArrowUpRight,
  Banknote,
  CarFront,
  Check,
  CheckCheck,
  ChevronRight,
  Clock3,
  Copy,
  CreditCard,
  MapPin,
  MessageSquareText,
  Phone,
  Route,
  ShieldAlert,
  Timer,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { RouteMap } from "@/components/map/RouteMap";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { serviceLabel, useClassName } from "@/lib/catalog";
import { cents, duration, km, money, STATUS_LABEL, statusTone, when } from "@/lib/format";
import { useRegulator } from "@/lib/scope";
import { useCancelTrip, useTrip } from "./hooks";
import { TripFareActions } from "./TripFareActions";
import { TripPaymentPanel } from "./TripPaymentPanel";
import { TripPricingRecord } from "./TripPricingRecord";

const LINE_LABEL: Record<string, string> = {
  BASE_FARE: "Trip fare",
  SURGE: "Busy-area surcharge",
  WAITING: "Waiting at pickup",
  ROUNDING: "Rounding",
  BOOKING_FEE: "Booking fee",
  FIXED_ROUTE: "Fixed fare",
  PREVIOUS_CANCELLATION: "Earlier cancellation fee",
  MINIMUM_FARE_ADJUSTMENT: "Minimum fare top-up",
  MANUAL_PRICE: "Price set by support",
};
const ENDED = [
  "COMPLETED",
  "CANCELLED_BY_RIDER",
  "CANCELLED_BY_DRIVER",
  "CANCELLED_BY_STAFF",
  "NO_DRIVER_FOUND",
  "EXPIRED",
];
const PAYMENT_METHOD: Record<string, string> = {
  CASH: "Cash",
  MOBILE_MONEY: "Mobile money",
  CARD: "Card",
  WALLET: "Wallet",
};

export function TripDetail({ id }: { id: string }) {
  const regulator = useRegulator();
  const { data: t, isLoading, error, refetch } = useTrip(id);
  const cancel = useCancelTrip();
  const me = useMe();
  const className = useClassName();
  const [confirming, setConfirming] = useState(false);
  const lines = [...(t?.quote_option?.lines ?? []), ...(t?.adjustments ?? [])].filter(
    (line) => Number(line.amount) !== 0,
  );
  const stops = [...(t?.stops ?? [])].sort((a, b) => a.seq - b.seq);
  const events = [...(t?.events ?? [])].sort((a, b) => a.seq - b.seq);
  const fare = t?.final_fare ?? t?.estimated_fare;
  const fees = t?.fees;

  return (
    <div className="pb-2">
      <nav aria-label="Breadcrumb" className="mb-5 flex items-center gap-2 pt-3 text-xs text-muted">
        <Link
          href="/trips"
          className="inline-flex items-center gap-1.5 transition-colors hover:text-brand-text"
        >
          <ArrowLeft size={13} aria-hidden="true" /> All trips
        </Link>
        <ChevronRight size={12} aria-hidden="true" />
        <span aria-current="page" className="font-medium text-ink">
          {t?.code ?? "Trip details"}
        </span>
      </nav>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-[30px] font-semibold tracking-tight sm:text-[34px]">
              {t ? `Trip ${t.code}` : "Trip details"}
            </h1>
            {t ? <Chip tone={statusTone(t.status)}>{STATUS_LABEL[t.status] ?? t.status}</Chip> : null}
          </div>
          <p className="mt-1.5 text-sm text-muted">
            {t ? `Requested ${when(t.requested_at)}` : "Route, people, payments and trip activity"}
          </p>
        </div>
        {t ? <CopyTripId id={t.id} /> : null}
      </div>

      {error ? (
        <div className="grid gap-3 rounded-xl border border-line bg-surface p-5">
          <ErrorNote error={error} />
          <Button variant="soft" className="justify-self-start" onClick={() => void refetch()}>
            Try again
          </Button>
        </div>
      ) : isLoading ? (
        <div role="status" aria-label="Loading trip details" className="grid gap-5">
          <Skeleton className="h-24" />
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,1fr)]">
            <Skeleton className="h-[540px]" />
            <Skeleton className="h-[540px]" />
          </div>
        </div>
      ) : !t ? (
        <Empty title="Trip not found" body="Return to all trips to find the trip you need." />
      ) : (
        <>
          <dl className="mb-5 grid grid-cols-2 overflow-hidden rounded-xl border border-line bg-surface lg:grid-cols-4">
            <Metric
              icon={<CarFront size={18} />}
              label="Service"
              value={`${serviceLabel(t.service)} · ${className(t.vehicle_class)}`}
            />
            <Metric
              icon={<Route size={18} />}
              label="Route distance"
              value={t.route_distance_m != null ? km(t.route_distance_m) : "Not available"}
            />
            <Metric
              icon={<Timer size={18} />}
              label="Est. travel time"
              value={t.route_duration_s != null ? duration(t.route_duration_s) : "Not available"}
            />
            <Metric
              icon={t.payment_method === "CASH" ? <Banknote size={18} /> : <CreditCard size={18} />}
              label="Payment method"
              value={PAYMENT_METHOD[t.payment_method] ?? readable(t.payment_method)}
            />
          </dl>

          {t.scheduled_for ? (
            <div className="mb-5 flex items-center gap-3 rounded-xl border border-warn/20 bg-warn/10 px-4 py-3 text-sm">
              <Clock3 size={17} className="shrink-0 text-warn" aria-hidden="true" />
              <p>
                <span className="font-semibold">Scheduled trip</span>
                <span className="text-muted">{` · ${when(t.scheduled_for)}`}</span>
              </p>
            </div>
          ) : null}

          <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,1fr)]">
            <div className="contents xl:grid xl:min-w-0 xl:gap-5">
              <section
                aria-labelledby="trip-route-heading"
                className="order-1 overflow-hidden rounded-xl border border-line bg-surface xl:order-none"
              >
                <div className="flex items-center justify-between gap-3 px-5 py-4">
                  <SectionHeading id="trip-route-heading" icon={<MapPin size={17} />}>
                    Trip route
                  </SectionHeading>
                  <span className="rounded-md bg-surface-2 px-2 py-1 text-xs font-medium text-muted">{`${stops.length} ${stops.length === 1 ? "stop" : "stops"}`}</span>
                </div>
                {stops.length ? (
                  <>
                    <RouteMap
                      stops={stops.map((s) => ({ kind: s.kind, ...s.location }))}
                      driver={t.driver_location?.location}
                      className="h-[280px] sm:h-[320px]"
                    />
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-line bg-surface-2/50 px-5 py-2.5 text-[11px] text-muted">
                      <span className="inline-flex items-center gap-1.5">
                        <span className="size-1.5 rounded-full bg-brand" />
                        Pickup
                      </span>
                      <span className="inline-flex items-center gap-1.5">
                        <span className="size-1.5 rounded-full bg-ink" />
                        Drop-off
                      </span>
                      <span className="ml-auto inline-flex items-center gap-1.5">
                        <span className="w-4 border-t-2 border-dashed border-brand/50" />
                        Stops connected for reference
                      </span>
                    </div>
                    <ol className="px-5 py-1">
                      {stops.map((stop, index) => (
                        <li key={stop.seq} className="relative flex gap-3 py-4">
                          {index < stops.length - 1 ? (
                            <span
                              aria-hidden="true"
                              className="absolute bottom-[-16px] left-[13px] top-[42px] border-l border-dashed border-line"
                            />
                          ) : null}
                          <span
                            aria-hidden="true"
                            className={`relative grid size-7 shrink-0 place-items-center rounded-full text-[11px] font-bold ${stop.kind === "PICKUP" ? "bg-brand-soft text-brand-text" : stop.kind === "DROPOFF" ? "bg-nav text-nav-ink" : "bg-surface-2 text-muted"}`}
                          >
                            {String.fromCharCode(65 + index)}
                          </span>
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center justify-between gap-1">
                              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted">
                                {stop.kind === "PICKUP"
                                  ? "Pickup location"
                                  : stop.kind === "DROPOFF"
                                    ? "Drop-off location"
                                    : "Additional stop"}
                              </p>
                              {stop.arrived_at ? (
                                <time dateTime={stop.arrived_at} className="tnum text-[11px] text-muted">
                                  {when(stop.arrived_at)}
                                </time>
                              ) : null}
                            </div>
                            <p
                              className={`mt-1 break-words text-sm font-medium ${stop.address ? "" : "tnum"}`}
                            >
                              {stop.address ||
                                `${stop.location.lat.toFixed(5)}, ${stop.location.lng.toFixed(5)}`}
                            </p>
                          </div>
                        </li>
                      ))}
                    </ol>
                  </>
                ) : (
                  <Empty
                    title="No route recorded"
                    body="Pickup and drop-off locations are not available for this trip."
                  />
                )}
              </section>

              <section
                aria-labelledby="trip-timeline-heading"
                className="order-4 rounded-xl border border-line bg-surface p-5 xl:order-none"
              >
                <div className="mb-5 flex items-center justify-between gap-2">
                  <SectionHeading id="trip-timeline-heading" icon={<Clock3 size={17} />}>
                    Activity timeline
                  </SectionHeading>
                  <span className="text-[11px] text-muted">{`${events.length} events`}</span>
                </div>
                {events.length ? (
                  <ol>
                    {events.map((event, index) => {
                      const latest = index === events.length - 1;
                      return (
                        <li key={event.seq} className="relative flex gap-3 pb-3 last:pb-0">
                          {!latest ? (
                            <span
                              className="absolute bottom-0 left-[10px] top-5 border-l border-line"
                              aria-hidden="true"
                            />
                          ) : null}
                          <span
                            aria-hidden="true"
                            className={`relative grid size-[21px] shrink-0 place-items-center rounded-full ${latest ? (t.status === "COMPLETED" ? "bg-good/15 text-good" : "bg-brand-soft text-brand-text") : "bg-surface-2 text-muted"}`}
                          >
                            {latest && t.status === "COMPLETED" ? (
                              <CheckCheck size={12} />
                            ) : (
                              <Check size={11} />
                            )}
                          </span>
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-baseline justify-between gap-x-2 gap-y-1">
                              <p className={`text-xs leading-5 ${latest ? "font-semibold" : "font-medium"}`}>
                                {eventLabel(event)}
                              </p>
                              <time
                                dateTime={event.at}
                                title={when(event.at)}
                                className="tnum text-[10px] leading-5 text-muted"
                              >
                                {new Date(event.at).toLocaleTimeString("en-GB", {
                                  hour: "2-digit",
                                  minute: "2-digit",
                                  second: "2-digit",
                                })}
                              </time>
                            </div>
                            <p className="text-[10px] text-muted">{`${readable(event.actor_kind)} · ${when(event.at)}`}</p>
                          </div>
                        </li>
                      );
                    })}
                  </ol>
                ) : (
                  <p className="py-4 text-sm text-muted">No events recorded yet.</p>
                )}
              </section>

              {t.notes || t.cancel_reason ? (
                <section
                  aria-labelledby="trip-notes-heading"
                  className="order-5 rounded-xl border border-line bg-surface p-5 xl:order-none"
                >
                  <SectionHeading id="trip-notes-heading" icon={<MessageSquareText size={17} />}>
                    Trip notes
                  </SectionHeading>
                  {t.notes ? (
                    <p className="mt-3 whitespace-pre-wrap break-words text-sm leading-relaxed text-muted">
                      {t.notes}
                    </p>
                  ) : null}
                  {t.cancel_reason ? (
                    <p className="mt-3 text-sm">
                      <span className="font-medium">Cancellation reason: </span>
                      <span className="text-muted">{t.cancel_reason}</span>
                    </p>
                  ) : null}
                </section>
              ) : null}

              {!ENDED.includes(t.status) && me.can("trips.cancel") ? (
                <section
                  aria-label="Trip support"
                  className="order-6 flex flex-wrap items-center justify-between gap-4 rounded-xl border border-critical/15 bg-surface p-5 xl:order-none"
                >
                  <div className="flex items-center gap-3">
                    <ShieldAlert size={19} className="shrink-0 text-critical" aria-hidden="true" />
                    <div>
                      <h2 className="text-sm font-semibold">Need to end this trip?</h2>
                      <p className="mt-1 text-xs text-muted">The rider and driver will be notified.</p>
                    </div>
                  </div>
                  <Button variant="danger" size="sm" onClick={() => setConfirming(true)}>
                    End trip
                  </Button>
                </section>
              ) : null}
              <ErrorNote error={cancel.error} />
            </div>

            <div className="contents xl:grid xl:min-w-0 xl:gap-5">
              <section
                aria-labelledby="trip-fare-heading"
                className="order-2 overflow-hidden rounded-xl border border-line bg-surface xl:order-none"
              >
                <div className="px-5 pb-5 pt-4">
                  <SectionHeading id="trip-fare-heading" icon={<CreditCard size={17} />}>
                    Fare breakdown
                  </SectionHeading>
                  <div className="my-5 rounded-lg bg-surface-2 p-4">
                    <p className="text-xs text-muted">
                      {t.final_fare != null ? "Final trip fare" : "Estimated trip fare"}
                    </p>
                    <p className="tnum mt-1.5 flex flex-wrap items-baseline gap-x-2">
                      <span className="text-sm font-medium text-muted">{t.currency}</span>
                      <span className="text-[36px] font-semibold leading-tight tracking-tight">
                        {Math.round(Number(fare)).toLocaleString("en-US")}
                      </span>
                    </p>
                    {t.final_fare != null && Number(t.estimated_fare) !== Number(t.final_fare) ? (
                      <p className="mt-2 text-xs text-muted">{`Original estimate ${money(t.estimated_fare, t.currency)}`}</p>
                    ) : null}
                  </div>
                  <dl className="grid gap-3 text-sm">
                    {lines.map((line, index) => (
                      <div
                        key={`${line.code}-${index}`}
                        className="flex items-baseline justify-between gap-3"
                      >
                        <dt className="text-muted">
                          {line.label?.en ?? LINE_LABEL[line.code] ?? readable(line.code)}
                        </dt>
                        <dd className="tnum shrink-0 font-medium">{money(line.amount, t.currency)}</dd>
                      </div>
                    ))}
                    <div className="flex justify-between gap-3 border-t border-line pt-3 font-semibold">
                      <dt>Total fare</dt>
                      <dd className="tnum">{money(fare, t.currency)}</dd>
                    </div>
                  </dl>
                  {fees ? (
                    <div className="mt-5 border-t border-line pt-4">
                      <h3 className="mb-3 text-[10px] font-semibold uppercase tracking-wider text-muted">
                        Earnings & fees
                      </h3>
                      <dl className="grid gap-3 text-xs">
                        <div className="flex justify-between gap-3 rounded-lg bg-brand-soft px-3 py-3 text-brand-text">
                          <dt className="flex items-center gap-2 font-medium">
                            <CarFront size={14} aria-hidden="true" />
                            Driver keeps
                          </dt>
                          <dd className="tnum font-semibold">{money(fees.driver_earnings, t.currency)}</dd>
                        </div>
                        <FareFact label="Commission" value={money(fees.commission, t.currency)} />
                        <FareFact label="VAT" value={`${t.currency} ${cents(fees.vat)}`} />
                        <FareFact label={regulator.levy} value={`${t.currency} ${cents(fees.levy)}`} />
                      </dl>
                    </div>
                  ) : null}
                  {Number(t.cancellation_fee) > 0 ? (
                    <p className="mt-4 rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted">{`Cancellation fee charged: ${money(t.cancellation_fee, t.currency)}`}</p>
                  ) : null}
                </div>
                <TripPaymentPanel trip={t} />
                <TripFareActions trip={t} />
                <TripPricingRecord trip={t} />
              </section>

              <section
                aria-labelledby="trip-people-heading"
                className="order-3 rounded-xl border border-line bg-surface p-5 xl:order-none"
              >
                <SectionHeading id="trip-people-heading" icon={<CarFront size={17} />}>
                  People on this trip
                </SectionHeading>
                <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                  <Person
                    role="Rider"
                    name={t.rider?.name}
                    phone={t.rider?.phone}
                    href={`/riders/${t.rider_id}`}
                  />
                  <Person
                    role="Driver"
                    name={t.driver?.name}
                    phone={t.driver?.phone}
                    href={t.driver_id ? `/drivers/${t.driver_id}` : undefined}
                    unassigned={!t.driver_id}
                  />
                </div>
              </section>
            </div>
          </div>
        </>
      )}
      <ReasonDialog
        open={confirming}
        title="End this trip?"
        body="The rider and driver are told the trip was ended by support."
        confirm="End trip"
        danger
        busy={cancel.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={(reason) =>
          t && cancel.mutate({ id: t.id, reason }, { onSuccess: () => setConfirming(false) })
        }
      />
    </div>
  );
}

function SectionHeading({
  id,
  icon,
  children,
}: {
  id: string;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <h2 id={id} className="flex items-center gap-2 text-sm font-semibold">
      <span className="text-muted" aria-hidden="true">
        {icon}
      </span>
      {children}
    </h2>
  );
}

function Metric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex min-w-0 items-center gap-3 border-line px-4 py-5 even:border-l max-lg:nth-[n+3]:border-t lg:not-first:border-l">
      <span
        aria-hidden="true"
        className="hidden size-9 shrink-0 place-items-center rounded-lg bg-surface-2 text-muted sm:grid"
      >
        {icon}
      </span>
      <div className="min-w-0">
        <dt className="text-[11px] text-muted">{label}</dt>
        <dd className="tnum mt-1 break-words text-sm font-semibold">{value}</dd>
      </div>
    </div>
  );
}

function Person({
  role,
  name,
  phone,
  href,
  unassigned = false,
}: {
  role: string;
  name?: string | null;
  phone?: string;
  href?: string;
  unassigned?: boolean;
}) {
  const initials = name
    ?.trim()
    .split(/\s+/)
    .map((part) => part[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
  return (
    <div className="min-w-0 rounded-lg border border-line p-3.5">
      <div className="mb-3 flex items-center justify-between gap-2">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-muted">{role}</p>
        {href ? (
          <Link
            href={href}
            aria-label={`View ${role.toLowerCase()} profile`}
            className="inline-flex items-center gap-1 text-[11px] font-medium text-brand-text hover:underline"
          >
            Profile
            <ArrowUpRight size={12} aria-hidden="true" />
          </Link>
        ) : null}
      </div>
      <div className="flex items-center gap-2.5">
        <span
          aria-hidden="true"
          className={`grid size-9 shrink-0 place-items-center rounded-full text-xs font-semibold ${role === "Rider" ? "bg-brand-soft text-brand-text" : "bg-surface-2 text-muted"}`}
        >
          {initials || (role === "Driver" ? <CarFront size={16} /> : "?")}
        </span>
        <p className="min-w-0 break-words text-sm font-semibold">
          {name || (unassigned ? "Not assigned" : "Name unavailable")}
        </p>
      </div>
      {phone ? (
        <a
          href={`tel:${phone}`}
          className="tnum mt-3 inline-flex items-center gap-2 text-xs text-muted transition-colors hover:text-brand-text"
        >
          <Phone size={12} aria-hidden="true" />
          {phone}
        </a>
      ) : (
        <p className="mt-3 text-xs text-muted">
          {unassigned ? "No driver was assigned to this trip." : "Phone number unavailable"}
        </p>
      )}
    </div>
  );
}

function FareFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-muted">{label}</dt>
      <dd className="tnum font-medium">{value}</dd>
    </div>
  );
}

function CopyTripId({ id }: { id: string }) {
  const [copiedId, setCopiedId] = useState<string>();
  const [failed, setFailed] = useState(false);
  const copied = copiedId === id;
  return (
    <div className="flex flex-col items-end gap-1">
      <Button
        size="sm"
        variant="soft"
        icon={copied ? <Check size={13} aria-hidden="true" /> : <Copy size={13} aria-hidden="true" />}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(id);
            setCopiedId(id);
            setFailed(false);
          } catch {
            setFailed(true);
          }
        }}
      >
        {copied ? "Trip ID copied" : "Copy trip ID"}
      </Button>
      <span role="status" className="max-w-full break-all text-xs text-muted">
        {failed ? `Copy this ID: ${id}` : copied ? "Copied to clipboard" : ""}
      </span>
    </div>
  );
}

const readable = (value: string) =>
  value
    .toLowerCase()
    .replace(/[._]/g, " ")
    .replace(/^\w/, (c) => c.toUpperCase());

function eventLabel(event: Schemas["TripEvent"]): string {
  if (event.type === "trip.price_set")
    return `Price set by support: ${String(event.payload?.from)} → ${String(event.payload?.to)}`;
  const status = event.payload?.status ?? event.payload?.to;
  if (typeof status === "string") return STATUS_LABEL[status] ?? readable(status);
  const labels: Record<string, string> = {
    requested: "Trip requested",
    offered: "Offer sent to driver",
    completed: "Trip completed",
  };
  const type = event.type.replace(/^trip\./, "");
  return labels[type] ?? readable(type);
}
