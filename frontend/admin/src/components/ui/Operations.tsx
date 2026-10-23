"use client";

import { ChevronLeft, ChevronRight, RefreshCw } from "lucide-react";
import { Button } from "./Button";
import { Delta } from "./Delta";
import { Skeleton } from "./States";

export function OperationsHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mb-6 flex flex-wrap items-center justify-between gap-4 pt-3">
      <div>
        <h1 className="text-[30px] font-semibold tracking-tight sm:text-[34px]">{title}</h1>
        <p className="mt-1.5 max-w-2xl text-sm leading-relaxed text-muted">{description}</p>
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}

export function UpdateIndicator({
  at,
  error,
  fetching = false,
  interval,
}: {
  at: number;
  error?: unknown;
  fetching?: boolean;
  interval: string;
}) {
  return (
    <div
      role="status"
      className="inline-flex items-center gap-2 rounded-lg border border-line bg-surface px-3 py-2 text-[11px] text-muted"
    >
      <span
        aria-hidden="true"
        className={`size-1.5 shrink-0 rounded-full ${error ? "bg-warn" : at ? "bg-good" : "bg-muted"}`}
      />
      <span>
        {error
          ? "Update failed"
          : !at
            ? "Loading data"
            : fetching
              ? "Updating"
              : `Refreshes every ${interval}`}
      </span>
      {at ? (
        <time
          dateTime={new Date(at).toISOString()}
          title="Last successful update"
          className="tnum border-l border-line pl-2"
        >
          {new Date(at).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" })}
        </time>
      ) : null}
    </div>
  );
}

export function MetricTile({
  label,
  value,
  note,
  icon,
  previous,
  numericValue,
  accent = false,
}: {
  label: string;
  value?: string;
  note: string;
  icon: React.ReactNode;
  previous?: number;
  numericValue?: number;
  accent?: boolean;
}) {
  return (
    <div
      className={`min-w-0 rounded-xl border p-4 sm:p-5 ${accent ? "border-brand/15 bg-brand-soft" : "border-line bg-surface"}`}
    >
      <div className="flex items-start justify-between gap-2">
        <p className="text-xs font-medium text-muted">{label}</p>
        <span
          aria-hidden="true"
          className={`grid size-8 shrink-0 place-items-center rounded-lg ${accent ? "bg-brand/10 text-brand-text" : "bg-surface-2 text-muted"}`}
        >
          {icon}
        </span>
      </div>
      {value === undefined ? (
        <Skeleton className="my-2 h-9 w-24" />
      ) : (
        <p
          className={`tnum mt-2 break-words text-[25px] font-semibold leading-tight tracking-tight sm:text-[30px] ${accent ? "text-brand-text" : ""}`}
        >
          {value}
        </p>
      )}
      <p className="mt-2 text-[11px] leading-relaxed text-muted">{note}</p>
      {previous !== undefined && numericValue !== undefined ? (
        <div className="mt-2 border-t border-line pt-2">
          <Delta now={numericValue} before={previous} />
        </div>
      ) : null}
    </div>
  );
}

export function OperationsPanel({
  title,
  description,
  action,
  children,
  className = "",
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section
      aria-label={title}
      className={`min-w-0 overflow-hidden rounded-xl border border-line bg-surface ${className}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-5 py-4">
        <div>
          <h2 className="text-sm font-semibold">{title}</h2>
          {description ? <p className="mt-1 text-xs text-muted">{description}</p> : null}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

export function FilterBar<T extends string>({
  value,
  options,
  onChange,
  label,
}: {
  value: T;
  options: readonly { value: T; label: string; count?: number }[];
  onChange: (value: T) => void;
  label: string;
}) {
  return (
    <div role="group" aria-label={label} className="flex max-w-full gap-1 overflow-x-auto p-1">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={value === option.value}
          onClick={() => onChange(option.value)}
          className={`inline-flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium transition-colors ${value === option.value ? "bg-brand-soft text-brand-text" : "text-muted hover:bg-surface-2 hover:text-ink"}`}
        >
          {option.label}
          {option.count !== undefined ? (
            <span
              className={`tnum rounded px-1.5 py-0.5 text-[10px] ${value === option.value ? "bg-brand/10" : "bg-surface-2"}`}
            >
              {option.count.toLocaleString("en-US")}
            </span>
          ) : null}
        </button>
      ))}
    </div>
  );
}

export function ListPagination({
  total,
  offset,
  pageSize,
  noun,
  onPage,
  busy,
}: {
  total: number;
  offset: number;
  pageSize: number;
  noun: string;
  onPage: (page: number) => void;
  busy?: boolean;
}) {
  const page = Math.floor(offset / pageSize);
  const pages = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-5 py-4 text-xs text-muted">
      <p className="tnum">
        {total
          ? `${Math.min(offset + 1, total)}–${Math.min(offset + pageSize, total)} of ${total.toLocaleString("en-US")} ${noun}`
          : `0 ${noun}`}
      </p>
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="soft"
          disabled={page === 0 || busy}
          onClick={() => onPage(page - 1)}
          icon={<ChevronLeft size={13} aria-hidden="true" />}
          aria-label="Previous page"
        >
          Previous
        </Button>
        <span className="tnum whitespace-nowrap px-1">{`${page + 1} / ${pages}`}</span>
        <Button
          size="sm"
          variant="soft"
          disabled={page + 1 >= pages || busy}
          onClick={() => onPage(page + 1)}
          icon={<ChevronRight size={13} aria-hidden="true" />}
          aria-label="Next page"
        >
          Next
        </Button>
      </div>
    </div>
  );
}

export function RecordsTable({ head, children }: { head: string[]; children: React.ReactNode }) {
  return (
    <div className="hidden max-h-[620px] overflow-auto md:block">
      <table className="w-full min-w-[920px] border-collapse text-sm">
        <thead className="sticky top-0 z-10 bg-surface-2">
          <tr>
            {head.map((label, index) => (
              <th
                key={`${label}-${index}`}
                scope="col"
                className={`whitespace-nowrap border-b border-line px-5 py-3 text-[10px] font-semibold uppercase tracking-wider text-muted ${label === "Fare" ? "text-right" : "text-left"}`}
              >
                {label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-line">{children}</tbody>
      </table>
    </div>
  );
}

export function Initials({ name, muted = false }: { name?: string | null; muted?: boolean }) {
  const initials = (name ?? "?")
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0])
    .join("")
    .toUpperCase();
  return (
    <span
      aria-hidden="true"
      className={`grid size-9 shrink-0 place-items-center rounded-full text-[11px] font-semibold ${muted ? "bg-surface-2 text-muted" : "bg-brand-soft text-brand-text"}`}
    >
      {initials || "?"}
    </span>
  );
}

export function RefreshButton({ onClick, busy }: { onClick: () => void; busy?: boolean }) {
  return (
    <Button
      variant="soft"
      size="sm"
      onClick={onClick}
      disabled={busy}
      icon={<RefreshCw size={13} className={busy ? "animate-spin" : ""} aria-hidden="true" />}
    >
      Refresh
    </Button>
  );
}

export function readPage(value?: string): number {
  const page = Number(value ?? 0);
  return Number.isSafeInteger(page) && page >= 0 ? page : 0;
}
