"use client";

import { ArrowUpRight, Banknote, CheckCheck, ChevronRight, Clock3, MapPin, Route } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense } from "react";
import { Chip } from "@/components/ui/Chip";
import {
  FilterBar,
  Initials,
  ListPagination,
  MetricTile,
  OperationsHeader,
  RecordsTable,
  RefreshButton,
  readPage,
} from "@/components/ui/Operations";
import { Search } from "@/components/ui/Search";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { useOverview } from "@/features/overview/hooks";
import type { Schemas } from "@/lib/api";
import { serviceLabel, useClassName } from "@/lib/catalog";
import { count, money, STATUS_LABEL, statusTone, when } from "@/lib/format";
import { useCountries, useScope } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { PAGE, type TripFilter, useTrips } from "./hooks";

const TABS = [
  { value: "all", label: "All trips" },
  { value: "active", label: "In progress" },
  { value: "scheduled", label: "Scheduled" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
] as const;
type Tab = (typeof TABS)[number]["value"];

export function TripsScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Trips />
    </Suspense>
  );
}

function Trips() {
  const url = useUrlState();
  const rawStatus = url.get("status");
  const status: Tab = TABS.find((tab) => tab.value === rawStatus)?.value ?? "all";
  const page = readPage(url.get("page"));
  const offset = page * PAGE;
  const filter: TripFilter = { status: status === "all" ? undefined : status, q: url.get("q"), offset };
  const trips = useTrips(filter);
  const overview = useOverview();
  const { scope } = useScope();
  const { data: countries } = useCountries();
  const currency =
    overview.data?.finance?.currency ??
    countries?.find((country) => country.code === scope.country)?.currency;
  const data = trips.data;
  const today = overview.data?.trips;
  const className = useClassName();
  const filtered = status !== "all" || !!url.get("q");
  const setPage = (next: number) => url.set({ page: next ? String(next) : undefined });
  return (
    <div>
      <OperationsHeader
        title="Trips"
        description="Every ride and delivery, from the first request to the final payment."
        actions={
          <>
            <Link
              href="/live"
              className="inline-flex h-9 items-center gap-2 rounded-lg bg-nav px-3 text-xs font-semibold text-nav-ink"
            >
              Live map <ArrowUpRight size={14} aria-hidden="true" />
            </Link>
            <RefreshButton
              busy={trips.isFetching}
              onClick={() => {
                void trips.refetch();
                void overview.refetch();
              }}
            />
          </>
        }
      />
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricTile
          label="Trips in this view"
          value={data ? count(data.total) : trips.error ? "—" : undefined}
          note={filtered ? "Matching your current filters" : "All recorded rides and deliveries"}
          icon={<Route size={17} />}
          accent
        />
        <MetricTile
          label="In progress"
          value={today ? count(today.active) : overview.error ? "—" : undefined}
          note="Across the selected country or region"
          icon={<Clock3 size={17} />}
        />
        <MetricTile
          label="Completed today"
          value={today ? count(today.completed) : overview.error ? "—" : undefined}
          note="Successfully finished since midnight"
          icon={<CheckCheck size={17} />}
        />
        <MetricTile
          label="Gross fares today"
          value={today ? money(today.gross_fares, currency) : overview.error ? "—" : undefined}
          note="Total fares on completed trips"
          icon={<Banknote size={17} />}
        />
      </div>
      <section
        aria-label="Trip directory"
        className="min-w-0 overflow-hidden rounded-xl border border-line bg-surface"
      >
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
          <div className="min-w-0 max-w-full">
            <FilterBar
              label="Filter trips by status"
              value={status}
              options={TABS}
              onChange={(value) => url.set({ status: value === "all" ? undefined : value, page: undefined })}
            />
          </div>
          <div className="w-full sm:w-64">
            <Search
              inset
              value={url.get("q")}
              onChange={(q) => url.set({ q: q || undefined, page: undefined })}
              placeholder="Search code, name or phone"
            />
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 px-5 py-3 text-[11px] text-muted">
          <span>{`${data ? count(data.total) : "…"} ${status === "all" ? "trips" : TABS.find((tab) => tab.value === status)?.label.toLowerCase() + " trips"}`}</span>
          <span>{trips.isFetching && !trips.isLoading ? "Updating results…" : "Newest requests first"}</span>
        </div>
        {trips.error ? (
          <div className="px-5 pb-4">
            <ErrorNote error={trips.error} />
          </div>
        ) : null}
        {trips.isLoading ? (
          <div className="p-5">
            <Skeleton className="h-80" />
          </div>
        ) : !data?.data.length ? (
          <Empty
            title={trips.error ? "Trips could not be loaded" : "No trips match this view"}
            body={trips.error ? "Use Refresh to try again." : "Try another status or clear your search."}
            icon={<Route size={22} />}
          />
        ) : (
          <>
            <RecordsTable head={["Trip", "Route", "Rider & driver", "Service", "Status", "Fare", ""]}>
              {data.data.map((trip) => (
                <TripRow
                  key={trip.id}
                  trip={trip}
                  type={`${serviceLabel(trip.service)} · ${className(trip.vehicle_class)}`}
                />
              ))}
            </RecordsTable>
            <ul className="max-h-[680px] divide-y divide-line overflow-y-auto md:hidden">
              {data.data.map((trip) => (
                <li key={trip.id}>
                  <TripCard
                    trip={trip}
                    type={`${serviceLabel(trip.service)} · ${className(trip.vehicle_class)}`}
                  />
                </li>
              ))}
            </ul>
          </>
        )}
        <ListPagination
          total={data?.total ?? 0}
          offset={offset}
          pageSize={PAGE}
          noun="trips"
          busy={trips.isFetching}
          onPage={setPage}
        />
      </section>
    </div>
  );
}

function TripRow({ trip: t, type }: { trip: Schemas["Trip"]; type: string }) {
  const href = `/trips/${t.id}`;
  const router = useRouter();
  return (
    <tr
      className="group cursor-pointer transition-colors hover:bg-surface-2/70"
      onClick={(event) => {
        if (!(event.target as HTMLElement).closest("a, button")) router.push(href);
      }}
    >
      <td className="px-5 py-4">
        <Link
          href={href}
          className="inline-flex items-center gap-2 font-mono text-xs font-semibold text-ink hover:text-brand-text"
        >
          <span className="grid size-7 place-items-center rounded-md bg-brand-soft text-brand-text">
            <Route size={13} aria-hidden="true" />
          </span>
          {t.code}
        </Link>
        <p className="tnum mt-1.5 whitespace-nowrap text-[10px] text-muted">
          {when(t.scheduled_for ?? t.requested_at)}
        </p>
      </td>
      <td className="max-w-[200px] px-5 py-4">
        <TripStops trip={t} />
      </td>
      <td className="px-5 py-4">
        <p className="max-w-40 truncate text-xs font-medium">
          {t.rider?.name || t.rider?.phone || "Rider unavailable"}
        </p>
        <p className="mt-1.5 max-w-40 truncate text-[11px] text-muted">
          {t.driver?.name || (t.driver_id ? "Driver assigned" : "No driver assigned")}
        </p>
      </td>
      <td className="px-5 py-4">
        <p className="whitespace-nowrap text-xs">{type}</p>
        <p className="mt-1.5 text-[11px] text-muted">{paymentLabel(t.payment_method)}</p>
      </td>
      <td className="px-5 py-4">
        <Chip tone={statusTone(t.status)}>{STATUS_LABEL[t.status] ?? t.status}</Chip>
      </td>
      <td className="tnum whitespace-nowrap px-5 py-4 text-right text-xs font-semibold">
        {money(t.final_fare ?? t.estimated_fare, t.currency)}
        <p className="mt-1.5 text-[10px] font-normal text-muted">
          {t.final_fare != null ? "Final fare" : "Estimated"}
        </p>
      </td>
      <td className="py-4 pr-4">
        <Link
          href={href}
          aria-label={`Open trip ${t.code}`}
          className="grid size-7 place-items-center rounded-lg text-muted transition-colors hover:bg-brand-soft hover:text-brand-text"
        >
          <ChevronRight size={15} aria-hidden="true" />
        </Link>
      </td>
    </tr>
  );
}

function TripCard({ trip: t, type }: { trip: Schemas["Trip"]; type: string }) {
  return (
    <Link href={`/trips/${t.id}`} className="block p-4 transition-colors hover:bg-surface-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-mono text-xs font-semibold">{t.code}</span>
        <Chip tone={statusTone(t.status)} wrap>
          {STATUS_LABEL[t.status] ?? t.status}
        </Chip>
      </div>
      <p className="tnum mt-2 text-[10px] text-muted">{`${when(t.scheduled_for ?? t.requested_at)} · ${type}`}</p>
      <div className="my-4">
        <TripStops trip={t} />
      </div>
      <div className="flex items-center justify-between gap-3 border-t border-line pt-3">
        <div className="flex min-w-0 items-center gap-2">
          <Initials name={t.rider?.name} />
          <span className="min-w-0 truncate text-xs">
            {t.rider?.name || t.rider?.phone || "Rider unavailable"}
          </span>
        </div>
        <div className="shrink-0 text-right">
          <p className="tnum text-sm font-semibold">{money(t.final_fare ?? t.estimated_fare, t.currency)}</p>
          <p className="mt-1 text-[10px] text-muted">{paymentLabel(t.payment_method)}</p>
        </div>
      </div>
    </Link>
  );
}

function TripStops({ trip }: { trip: Schemas["Trip"] }) {
  const stops = [...trip.stops].sort((a, b) => a.seq - b.seq);
  const pickup = stops.find((stop) => stop.kind === "PICKUP");
  const dropoff = stops.findLast((stop) => stop.kind === "DROPOFF");
  const label = (stop?: Schemas["TripStop"]) =>
    stop
      ? stop.address || `${stop.location.lat.toFixed(4)}, ${stop.location.lng.toFixed(4)}`
      : "Location unavailable";
  return (
    <div className="grid gap-2 text-[11px]">
      <p title={label(pickup)} className="flex items-center gap-2">
        <span aria-hidden="true" className="size-1.5 shrink-0 rounded-full bg-brand" />
        <span className="truncate">{label(pickup)}</span>
      </p>
      <p title={label(dropoff)} className="flex items-center gap-2 text-muted">
        <MapPin size={10} className="shrink-0" aria-hidden="true" />
        <span className="truncate">{label(dropoff)}</span>
      </p>
    </div>
  );
}

const paymentLabel = (method: string) =>
  ({ CASH: "Cash", MOBILE_MONEY: "Mobile money", CARD: "Card", WALLET: "Wallet" })[method] ?? method;
