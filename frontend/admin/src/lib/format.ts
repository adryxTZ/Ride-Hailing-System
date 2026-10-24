/** Formatting for figures: money, counts, durations, times. Pure functions. */

/** "TZS 15,000" (whole units; tax lines use cents()). */
export function money(v?: string | number | null, currency = "TZS"): string {
  const n = Number(v ?? 0);
  return `${currency} ${Math.round(n).toLocaleString("en-US")}`;
}

/** "5,794.16" with cents (tax lines are reported exactly). */
export function cents(v?: string | number | null): string {
  return Number(v ?? 0).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export const count = (n?: number) => (n ?? 0).toLocaleString("en-US");

/** "86%". */
export const percent = (r?: number) => `${Math.round((r ?? 0) * 100)}%`;

/** "3 min", "1 h 05 min". */
export function duration(s: number): string {
  const m = Math.max(0, Math.round(s / 60));
  if (m < 60) return `${m} min`;
  return `${Math.floor(m / 60)} h ${String(m % 60).padStart(2, "0")} min`;
}

/** "Today 14:05", "Yesterday 09:30", "12 Oct 08:01". */
export function when(iso?: string | null, now = new Date()): string {
  if (!iso) return "";
  const d = new Date(iso);
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(now) - day(d)) / 86_400_000);
  const time = `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  if (diff === 0) return `Today ${time}`;
  if (diff === 1) return `Yesterday ${time}`;
  return `${d.getDate()} ${d.toLocaleString("en-US", { month: "short" })} ${time}`;
}

/** "Completed", "Driver on the way" … for a trip status. */
export const STATUS_LABEL: Record<string, string> = {
  SCHEDULED: "Scheduled",
  REQUESTED: "Looking for a driver",
  OFFERED: "Offered to a driver",
  ACCEPTED: "Driver accepted",
  EN_ROUTE_TO_PICKUP: "Driver on the way",
  ARRIVED_AT_PICKUP: "At pickup",
  IN_TRANSIT: "On the trip",
  ARRIVED_AT_DROPOFF: "At drop-off",
  AWAITING_PAYMENT: "Collecting payment",
  COMPLETED: "Completed",
  CANCELLED_BY_RIDER: "Cancelled by rider",
  CANCELLED_BY_DRIVER: "Cancelled by driver",
  CANCELLED_BY_STAFF: "Ended by staff",
  NO_DRIVER_FOUND: "No driver found",
  EXPIRED: "Did not start",
};

/** Tone for a status chip. */
export function statusTone(s: string): "ok" | "live" | "warn" | "bad" | "muted" {
  if (s === "COMPLETED") return "ok";
  if (s.startsWith("CANCELLED") || s === "EXPIRED") return "muted";
  if (s === "NO_DRIVER_FOUND") return "bad";
  if (s === "SCHEDULED") return "warn";
  return "live";
}

/** Below this, waits show a tenth of a second ("0.4 s"). */
const SHORT_WAIT_S = 10;

/** "12 s", "3 min" — short waits such as how fast a driver answers. */
export function seconds(s?: number | null): string {
  if (s === undefined || s === null) return "None yet";
  if (s < SHORT_WAIT_S) return `${Math.max(0, s).toFixed(1)} s`;
  return s < 60 ? `${Math.round(s)} s` : duration(s);
}

/** "12.4 km" from metres. */
export const km = (m?: number) =>
  `${((m ?? 0) / 1000).toLocaleString("en-US", { maximumFractionDigits: 1 })} km`;

const KB = 1024;
/** "240 KB", "1.2 MB" for a file. */
export function fileSize(bytes?: number | null): string {
  if (!bytes) return "";
  if (bytes < KB * KB) return `${Math.max(1, Math.round(bytes / KB))} KB`;
  return `${(bytes / (KB * KB)).toLocaleString("en-US", { maximumFractionDigits: 1 })} MB`;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** "23 Oct 2026, 18:25" in local time, the same on the server and in the browser (no locale APIs). */
export function dateTime(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}, ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
