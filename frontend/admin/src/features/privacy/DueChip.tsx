"use client";
/**
 * DueChip — when an open request is due, with a word as well as a colour:
 * "Overdue" (critical), "Due soon" (warn) or just the date. Closed requests
 * show the date muted, since the deadline no longer matters.
 */
import { Chip } from "@/components/ui/Chip";
import { dateTime } from "@/lib/format";
import { OPEN_STATUSES, type PrivacyRequest } from "./hooks";

/** Open requests due within this many days are marked "Due soon". */
const DUE_SOON_DAYS = 3;
const DAY_MS = 86_400_000;

export function DueChip({ request }: { request: PrivacyRequest }) {
  const open = OPEN_STATUSES.includes(request.status);
  const date = dateTime(request.due_at);
  if (!open) return <span className="tnum whitespace-nowrap text-[11px] text-muted">{date}</span>;
  if (request.overdue)
    return (
      <span className="inline-flex flex-wrap items-center gap-1.5">
        <Chip tone="bad">Overdue</Chip>
        <span className="tnum whitespace-nowrap text-[11px] font-semibold text-critical">{date}</span>
      </span>
    );
  const soon = new Date(request.due_at).getTime() - Date.now() < DUE_SOON_DAYS * DAY_MS;
  if (soon)
    return (
      <span className="inline-flex flex-wrap items-center gap-1.5">
        <Chip tone="warn">Due soon</Chip>
        <span className="tnum whitespace-nowrap text-[11px] text-warn">{date}</span>
      </span>
    );
  return <span className="tnum whitespace-nowrap text-[11px]">{date}</span>;
}
