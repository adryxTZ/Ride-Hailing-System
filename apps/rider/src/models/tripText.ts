/** How trips read in lists: status words, dates and the fare shown (cancelled trips cost nothing). */
import { formatMoney } from "@/models/format";
import type { Trip } from "@/services/trips";

const STATUS_WORD: Record<string, string> = {
  COMPLETED: "Completed",
  CANCELLED_BY_RIDER: "Cancelled",
  CANCELLED_BY_DRIVER: "Cancelled by driver",
  CANCELLED_BY_STAFF: "Ended by support",
  NO_DRIVER_FOUND: "No driver found",
  EXPIRED: "Could not start",
  SCHEDULED: "Scheduled",
};

export function statusWord(t: Trip): string | undefined {
  return STATUS_WORD[t.status] ?? "In progress";
}

/** "Today, 12:57 pm", "Yesterday, 5:52 pm" or "12 Oct, 8:01 am". */
export function whenText(iso: string, now = new Date()): string {
  const d = new Date(iso);
  const h = d.getHours();
  const time = `${h % 12 === 0 ? 12 : h % 12}:${String(d.getMinutes()).padStart(2, "0")} ${h < 12 ? "am" : "pm"}`;
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(now) - day(d)) / 86_400_000);
  if (diff === 0) return `Today, ${time}`;
  if (diff === 1) return `Yesterday, ${time}`;
  return `${d.getDate()} ${d.toLocaleString("en-US", { month: "short" })}, ${time}`;
}

/** What the rider paid: the final fare for completed trips, nothing for trips that never happened. */
export function paidText(t: Trip): string {
  const paid = t.status === "COMPLETED" ? (t.final_fare ?? t.estimated_fare) : 0;
  return `${t.currency} ${formatMoney(paid)}`;
}

export const destinationOf = (t: Trip) =>
  t.stops.find((s) => s.kind === "DROPOFF")?.address ?? "Pinned location";
