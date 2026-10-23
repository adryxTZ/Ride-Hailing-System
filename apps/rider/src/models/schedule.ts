/**
 * schedule — the pickup-time choices for booking ahead. Mirrors the server's
 * rules (30 minutes to 7 days ahead) on a 15-minute grid. Pure functions.
 */
export const MIN_LEAD_MIN = 30;
export const MAX_AHEAD_DAYS = 7;
export const STEP_MIN = 15;
const MS_PER_MIN = 60_000;
const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** Midnight of the day containing `d` (local time). */
function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

/** The earliest pickup allowed, rounded up to the grid. */
export function earliestPickup(now: Date): Date {
  const t = now.getTime() + MIN_LEAD_MIN * MS_PER_MIN;
  const step = STEP_MIN * MS_PER_MIN;
  return new Date(Math.ceil(t / step) * step);
}

/** Selectable days: today through MAX_AHEAD_DAYS ahead, labelled "Today", "Tomorrow", "Wed 22". */
export function dayOptions(now: Date): { key: string; label: string; date: Date }[] {
  const first = startOfDay(earliestPickup(now));
  return Array.from({ length: MAX_AHEAD_DAYS }, (_, i) => {
    const date = new Date(first.getFullYear(), first.getMonth(), first.getDate() + i);
    return { key: date.toDateString(), label: dayLabel(date, now), date };
  });
}

/** Pickup times on one day, every STEP_MIN minutes, within the allowed range. */
export function timeSlots(day: Date, now: Date): Date[] {
  const earliest = earliestPickup(now).getTime();
  const latest = now.getTime() + MAX_AHEAD_DAYS * 24 * 60 * MS_PER_MIN;
  const out: Date[] = [];
  for (let m = 0; m < 24 * 60; m += STEP_MIN) {
    const t = startOfDay(day).getTime() + m * MS_PER_MIN;
    if (t >= earliest && t <= latest) out.push(new Date(t));
  }
  return out;
}

function dayLabel(d: Date, now: Date): string {
  const diff = Math.round((startOfDay(d).getTime() - startOfDay(now).getTime()) / (24 * 60 * MS_PER_MIN));
  if (diff === 0) return "Today";
  if (diff === 1) return "Tomorrow";
  return `${DAYS[d.getDay()]} ${d.getDate()}`;
}

/** "18:30". */
export function clock(d: Date): string {
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

/** "Today 18:30", "Tomorrow 07:15", "Wed 22 Oct 09:00". */
export function pickupLabel(iso: string, now = new Date()): string {
  const d = new Date(iso);
  const label = dayLabel(d, now);
  const day = label === "Today" || label === "Tomorrow" ? label : `${label} ${MONTHS[d.getMonth()]}`;
  return `${day} ${clock(d)}`;
}
