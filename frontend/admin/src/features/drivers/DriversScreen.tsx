"use client";

import { CarFront, ChevronRight, Clock3, ShieldCheck, Star, Users, UserX } from "lucide-react";
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
import type { Schemas } from "@/lib/api";
import { useClassName } from "@/lib/catalog";
import { count, when } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { DRIVER_PAGE, useDrivers } from "./hooks";
import { DRIVER_STATUS } from "./labels";

const TABS = ["ALL", "APPROVED", "PENDING_REVIEW", "ONBOARDING", "REJECTED", "SUSPENDED"] as const;
type Tab = (typeof TABS)[number];
const LABEL: Record<Tab, string> = {
  ALL: "All drivers",
  APPROVED: "Approved",
  PENDING_REVIEW: "Awaiting review",
  ONBOARDING: "Signing up",
  REJECTED: "Needs changes",
  SUSPENDED: "Suspended",
};

export function DriversScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Drivers />
    </Suspense>
  );
}

function Drivers() {
  const url = useUrlState();
  const tab = TABS.find((value) => value === url.get("status")) ?? "PENDING_REVIEW";
  const page = readPage(url.get("page"));
  const offset = page * DRIVER_PAGE;
  const drivers = useDrivers(tab === "ALL" ? undefined : tab, url.get("q"), offset);
  const { data, isLoading, error } = drivers;
  const className = useClassName();
  const counts = data?.counts;
  const total = Object.values(counts ?? {}).reduce((sum, value) => sum + value, 0);
  return (
    <div>
      <OperationsHeader
        title="Drivers"
        description="Manage your driver network, review applications, and keep account details within reach."
        actions={<RefreshButton busy={drivers.isFetching} onClick={() => void drivers.refetch()} />}
      />
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricTile
          label="Registered drivers"
          value={counts ? count(total) : error ? "—" : undefined}
          note="Across all application statuses"
          icon={<Users size={17} />}
        />
        <MetricTile
          label="Approved"
          value={counts ? count(counts.APPROVED) : error ? "—" : undefined}
          note="Applications approved to drive"
          icon={<ShieldCheck size={17} />}
          accent
        />
        <MetricTile
          label="Awaiting review"
          value={counts ? count(counts.PENDING_REVIEW) : error ? "—" : undefined}
          note="Submitted applications to check"
          icon={<Clock3 size={17} />}
        />
        <MetricTile
          label="Suspended"
          value={counts ? count(counts.SUSPENDED) : error ? "—" : undefined}
          note="Driver access is currently paused"
          icon={<UserX size={17} />}
        />
      </div>
      <section
        aria-label="Driver directory"
        className="min-w-0 overflow-hidden rounded-xl border border-line bg-surface"
      >
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
          <div className="min-w-0 max-w-full">
            <FilterBar
              label="Filter drivers by status"
              value={tab}
              options={TABS.map((value) => ({
                value,
                label: LABEL[value],
                count: counts ? (value === "ALL" ? total : (counts[value] ?? 0)) : undefined,
              }))}
              onChange={(value) =>
                url.set({ status: value === "PENDING_REVIEW" ? undefined : value, page: undefined })
              }
            />
          </div>
          <div className="w-full sm:w-64">
            <Search
              inset
              value={url.get("q")}
              onChange={(q) => url.set({ q: q || undefined, page: undefined })}
              placeholder="Search name, phone or plate"
            />
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 px-5 py-3 text-[11px] text-muted">
          <span>
            {data
              ? `${count(data.total)} ${tab === "ALL" ? "drivers" : LABEL[tab].toLowerCase() + " drivers"}`
              : "Loading drivers…"}
          </span>
          <span>
            {drivers.isFetching && !isLoading ? "Updating results…" : "Open a profile to view details"}
          </span>
        </div>
        {error ? (
          <div className="px-5 pb-4">
            <ErrorNote error={error} />
          </div>
        ) : null}
        {isLoading ? (
          <div className="p-5">
            <Skeleton className="h-80" />
          </div>
        ) : !data?.data.length ? (
          <Empty
            title={
              error
                ? "Drivers could not be loaded"
                : tab === "PENDING_REVIEW" && !url.get("q")
                  ? "You’re all caught up"
                  : "No drivers match this view"
            }
            body={
              error
                ? "Use Refresh to try again."
                : tab === "PENDING_REVIEW" && !url.get("q")
                  ? "New applications will appear here when drivers submit them."
                  : "Try another status or clear your search."
            }
            icon={<Users size={22} />}
          />
        ) : (
          <>
            <RecordsTable
              head={["Driver", "Vehicle", "Status", "Completed trips", "Rating", "Submitted / joined", ""]}
            >
              {data.data.map((driver) => (
                <DriverRow
                  key={driver.user_id}
                  driver={driver}
                  type={driver.vehicle_class ? className(driver.vehicle_class) : undefined}
                />
              ))}
            </RecordsTable>
            <ul className="max-h-[680px] divide-y divide-line overflow-y-auto md:hidden">
              {data.data.map((driver) => (
                <li key={driver.user_id}>
                  <DriverCard
                    driver={driver}
                    type={driver.vehicle_class ? className(driver.vehicle_class) : undefined}
                  />
                </li>
              ))}
            </ul>
          </>
        )}
        <ListPagination
          total={data?.total ?? 0}
          offset={offset}
          pageSize={DRIVER_PAGE}
          noun="drivers"
          busy={drivers.isFetching}
          onPage={(next) => url.set({ page: next ? String(next) : undefined })}
        />
      </section>
      <p className="mt-3 flex items-start gap-2 text-[11px] leading-relaxed text-muted">
        <ShieldCheck size={13} className="mt-0.5 shrink-0" aria-hidden="true" />
        Approval, suspension and document decisions are recorded in the audit log.
      </p>
    </div>
  );
}

function DriverRow({ driver: d, type }: { driver: Schemas["DriverRow"]; type?: string }) {
  const router = useRouter();
  return (
    <tr
      className="cursor-pointer transition-colors hover:bg-surface-2/70"
      onClick={(event) => {
        if (!(event.target as HTMLElement).closest("a, button")) router.push(`/drivers/${d.user_id}`);
      }}
    >
      <td className="px-5 py-4">
        <Link href={`/drivers/${d.user_id}`} className="group flex items-center gap-3">
          <Initials name={d.full_name} />
          <span className="min-w-0">
            <span className="block max-w-44 truncate text-xs font-semibold transition-colors group-hover:text-brand-text">
              {d.full_name || "Name unavailable"}
            </span>
            <span className="tnum mt-1 block text-[11px] text-muted">{d.phone}</span>
          </span>
        </Link>
      </td>
      <td className="px-5 py-4">
        {d.plate ? (
          <div className="flex items-center gap-2.5">
            <span className="grid size-8 place-items-center rounded-lg bg-surface-2 text-muted">
              <CarFront size={15} aria-hidden="true" />
            </span>
            <div>
              <p className="whitespace-nowrap font-mono text-xs font-medium">{d.plate}</p>
              <p className="mt-1 text-[11px] text-muted">{type || "Class unavailable"}</p>
            </div>
          </div>
        ) : (
          <span className="text-xs text-muted">No vehicle added</span>
        )}
      </td>
      <td className="px-5 py-4">
        <Chip tone={DRIVER_STATUS[d.status]?.tone}>{DRIVER_STATUS[d.status]?.label ?? d.status}</Chip>
      </td>
      <td className="tnum px-5 py-4 text-sm font-semibold">{count(d.trips_completed)}</td>
      <td className="px-5 py-4">
        <Rating rating={d.rating} />
      </td>
      <td className="tnum whitespace-nowrap px-5 py-4 text-[11px] text-muted">
        {when(d.submitted_at ?? d.created_at)}
      </td>
      <td className="py-4 pr-4">
        <Link
          href={`/drivers/${d.user_id}`}
          aria-label={`Open driver ${d.full_name || d.phone}`}
          className="grid size-7 place-items-center rounded-lg text-muted hover:bg-brand-soft hover:text-brand-text"
        >
          <ChevronRight size={15} aria-hidden="true" />
        </Link>
      </td>
    </tr>
  );
}

function DriverCard({ driver: d, type }: { driver: Schemas["DriverRow"]; type?: string }) {
  return (
    <Link href={`/drivers/${d.user_id}`} className="block p-4 transition-colors hover:bg-surface-2">
      <div className="flex items-center gap-3">
        <Initials name={d.full_name} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold">{d.full_name || "Name unavailable"}</p>
          <p className="tnum mt-1 text-[11px] text-muted">{d.phone}</p>
        </div>
        <ChevronRight size={15} className="shrink-0 text-muted" aria-hidden="true" />
      </div>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
        <Chip tone={DRIVER_STATUS[d.status]?.tone}>{DRIVER_STATUS[d.status]?.label ?? d.status}</Chip>
        <span className="text-[11px] text-muted">
          {d.plate ? `${type || "Vehicle"} · ${d.plate}` : "No vehicle added"}
        </span>
      </div>
      <div className="mt-3 flex items-center justify-between border-t border-line pt-3">
        <span className="tnum text-xs">
          <span className="font-semibold">{count(d.trips_completed)}</span>
          <span className="text-muted"> trips completed</span>
        </span>
        <Rating rating={d.rating} />
      </div>
    </Link>
  );
}

function Rating({ rating }: { rating?: number }) {
  return rating != null && rating > 0 ? (
    <span className="tnum inline-flex items-center gap-1.5 text-xs font-semibold">
      <Star size={12} className="fill-warn/20 text-warn" aria-hidden="true" />
      {rating.toFixed(1)}
    </span>
  ) : (
    <span className="text-[11px] text-muted">Not rated</span>
  );
}
