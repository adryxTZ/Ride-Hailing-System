"use client";
/**
 * TripsPanel — the right-hand column of a person page: every trip they rode or
 * drove, newest first, searchable (code, the other person, an address) and
 * filterable by state. Trips in progress are pinned on top and refresh live.
 */
import { ChevronLeft, ChevronRight } from "lucide-react";
import { motion } from "motion/react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { enter } from "@/components/ui/motion";
import { Search } from "@/components/ui/Search";
import { Empty, Skeleton } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { useClassName } from "@/lib/catalog";
import { money, STATUS_LABEL, statusTone, when } from "@/lib/format";
import { PERSON_TRIPS_PAGE, type Role, type TripGroup, usePersonTrips } from "./hooks";
import { PAYMENT_LABEL } from "./labels";

const FILTERS = [
  { value: "all", label: "All" },
  { value: "active", label: "In progress" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
  { value: "scheduled", label: "Scheduled" },
] as const;
type Filter = (typeof FILTERS)[number]["value"];

export function TripsPanel({ role, id, index }: { role: Role; id: string; index: number }) {
  const [filter, setFilter] = useState<Filter>("all");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(0);
  const status = filter === "all" ? undefined : (filter as TripGroup);
  const list = usePersonTrips(role, id, status, page * PERSON_TRIPS_PAGE, q);
  // The live trips sit above the list on the first page of "All".
  const live = usePersonTrips(role, id, "active", 0, q);
  const pinned = filter === "all" && page === 0 ? (live.data?.data ?? []) : [];
  const rows = (list.data?.data ?? []).filter((t) => !pinned.some((p) => p.id === t.id));
  const pages = Math.max(1, Math.ceil((list.data?.total ?? 0) / PERSON_TRIPS_PAGE));
  return (
    <motion.aside
      {...enter(index)}
      aria-label="Trips"
      className="min-w-0 rounded-[10px] bg-surface p-5 xl:sticky xl:top-4 xl:flex xl:max-h-[calc(100vh-2rem)] xl:flex-col"
    >
      <div className="mb-3 flex items-baseline justify-between gap-2">
        <h2 className="text-[15px] font-semibold">Trips</h2>
        <span className="tnum text-sm text-muted">{`${list.data?.total ?? 0} found`}</span>
      </div>
      <Search
        inset
        value={q}
        onChange={(v) => {
          setQ(v);
          setPage(0);
        }}
        placeholder={`Code, ${role === "rider" ? "driver" : "rider"} or address`}
      />
      <div className="mt-3 flex flex-wrap gap-1.5" role="group" aria-label="Filter trips">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            type="button"
            aria-pressed={filter === f.value}
            onClick={() => {
              setFilter(f.value);
              setPage(0);
            }}
            className={`rounded-[8px] px-2.5 py-1 text-xs font-medium transition-colors duration-150 ${filter === f.value ? "bg-nav text-nav-ink" : "bg-surface-2 text-muted hover:text-ink"}`}
          >
            {f.label}
          </button>
        ))}
      </div>
      <div className="no-scrollbar -mx-2 mt-3 min-h-0 flex-1 overflow-y-auto px-2">
        {list.isLoading ? (
          <Skeleton className="h-64" />
        ) : !rows.length && !pinned.length ? (
          <Empty title="No trips" body={q ? "Nothing matches that search." : "Nothing in this filter."} />
        ) : (
          <ul className="grid gap-1.5">
            {pinned.map((t) => (
              <TripItem key={t.id} trip={t} role={role} live />
            ))}
            {rows.map((t) => (
              <TripItem key={t.id} trip={t} role={role} />
            ))}
          </ul>
        )}
      </div>
      {pages > 1 ? (
        <div className="mt-3 flex items-center justify-between text-sm text-muted">
          <Button
            variant="soft"
            size="sm"
            disabled={page === 0}
            onClick={() => setPage(page - 1)}
            icon={<ChevronLeft size={14} aria-hidden="true" />}
          >
            Newer
          </Button>
          <span className="tnum">{`${page + 1} / ${pages}`}</span>
          <Button
            variant="soft"
            size="sm"
            disabled={page + 1 >= pages}
            onClick={() => setPage(page + 1)}
            icon={<ChevronRight size={14} aria-hidden="true" />}
          >
            Older
          </Button>
        </div>
      ) : null}
    </motion.aside>
  );
}

/** A stop's address, or its coordinates when the trip carried no name. */
function stopName(s?: Schemas["Trip"]["stops"][number]): string | undefined {
  if (!s) return undefined;
  return s.address || `${s.location.lat.toFixed(4)}, ${s.location.lng.toFixed(4)}`;
}

/** One trip: code and state, when, the route, the other person, payment and fare. */
function TripItem({ trip: t, role, live = false }: { trip: Schemas["Trip"]; role: Role; live?: boolean }) {
  const className = useClassName();
  const from = stopName(t.stops[0]);
  const to = stopName(t.stops[t.stops.length - 1]);
  const other = role === "rider" ? t.driver?.name : (t.rider?.name ?? t.rider?.phone);
  return (
    <li>
      <Link
        href={`/trips/${t.id}`}
        className={`block rounded-[8px] px-3 py-2.5 transition-colors duration-150 hover:bg-surface-2 ${live ? "bg-brand-soft" : "bg-surface-2/60"}`}
      >
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs font-semibold">{t.code}</span>
          <Chip tone={statusTone(t.status)}>{STATUS_LABEL[t.status] ?? t.status}</Chip>
          <span className="tnum ml-auto whitespace-nowrap text-sm font-semibold">
            {money(t.final_fare ?? t.estimated_fare, t.currency)}
          </span>
        </div>
        <p className="mt-1 truncate text-sm">{[from, to].filter(Boolean).join(" → ") || "No stops"}</p>
        <p className="tnum mt-0.5 truncate text-xs text-muted">
          {[
            when(t.scheduled_for ?? t.requested_at),
            className(t.vehicle_class),
            other ?? (role === "rider" ? "No driver" : undefined),
            t.payment_method === "CASH" ? "Cash" : "Mobile money",
            t.payment_status ? PAYMENT_LABEL[t.payment_status] : undefined,
          ]
            .filter(Boolean)
            .join(" · ")}
        </p>
      </Link>
    </li>
  );
}
