/**
 * Privacy requests as the person sees them: types from the contract, and pure
 * display rules (labels, status tone, the date lines). No I/O, no React.
 */
import type { components } from "@platform/api-client";

export type PrivacyRequest = components["schemas"]["PrivacyRequest"];
export type PrivacyKind = PrivacyRequest["kind"];
export type PrivacyStatus = PrivacyRequest["status"];

/** Colour role of a status chip; the card maps it to a theme colour. */
export type StatusTone = "info" | "ok" | "bad" | "muted";

/** Shown when the person asks to delete the account, before sending. */
export const DELETE_CONFIRMATION =
  "This removes your name, phone, places and contacts. Trip and payment records are kept as the law requires.";

const KIND_TITLE: Record<PrivacyKind, string> = {
  EXPORT: "Copy of my data",
  DELETE: "Account deletion",
};

const STATUS_LABEL: Record<PrivacyStatus, string> = {
  REQUESTED: "Requested",
  IN_PROGRESS: "In progress",
  READY: "Ready",
  DONE: "Done",
  REJECTED: "Declined",
  CANCELLED: "Cancelled",
};

const STATUS_TONE: Record<PrivacyStatus, StatusTone> = {
  REQUESTED: "info",
  IN_PROGRESS: "info",
  READY: "ok",
  DONE: "ok",
  REJECTED: "bad",
  CANCELLED: "muted",
};

/** Statuses during which a second request of the same kind is refused. */
const OPEN_STATUSES: readonly PrivacyStatus[] = ["REQUESTED", "IN_PROGRESS"];

const DATE_FORMAT: Intl.DateTimeFormatOptions = { day: "numeric", month: "short", year: "numeric" };
const TIME_FORMAT: Intl.DateTimeFormatOptions = {
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
};
// App copy ships in English for now (the app translation layer is a later slice).
const LOCALE = "en-GB";

export function kindTitle(kind: PrivacyKind): string {
  return KIND_TITLE[kind];
}

export function statusLabel(status: PrivacyStatus): string {
  return STATUS_LABEL[status];
}

export function statusTone(status: PrivacyStatus): StatusTone {
  return STATUS_TONE[status];
}

/** "23 Oct 2026" */
export function formatDay(iso: string): string {
  return new Date(iso).toLocaleDateString(LOCALE, DATE_FORMAT);
}

/** "23 Oct, 14:30" */
export function formatMoment(iso: string): string {
  return new Date(iso).toLocaleString(LOCALE, TIME_FORMAT);
}

/** "Requested on {date}. We will reply by {due}." */
export function requestedLine(r: PrivacyRequest): string {
  return `Requested on ${formatDay(r.created_at)}. We will reply by ${formatDay(r.due_at)}.`;
}

/** "Ready to download until {time}." for a READY export, else null. */
export function readyLine(r: PrivacyRequest): string | null {
  if (!canDownload(r) || !r.download_expires_at) return null;
  return `Ready to download until ${formatMoment(r.download_expires_at)}.`;
}

/** A READY export whose link has not expired yet. */
export function canDownload(r: PrivacyRequest, now = Date.now()): boolean {
  if (r.kind !== "EXPORT" || r.status !== "READY") return false;
  return !r.download_expires_at || new Date(r.download_expires_at).getTime() > now;
}

/** Only a request nobody has picked up yet can be withdrawn. */
export function canCancel(r: PrivacyRequest): boolean {
  return r.status === "REQUESTED";
}

/** Is a request of this kind already waiting for staff? */
export function hasOpen(requests: readonly PrivacyRequest[], kind: PrivacyKind): boolean {
  return requests.some((r) => r.kind === kind && OPEN_STATUSES.includes(r.status));
}

/** Newest first. */
export function sortRequests(requests: readonly PrivacyRequest[]): PrivacyRequest[] {
  return [...requests].sort((a, b) => b.created_at.localeCompare(a.created_at));
}
