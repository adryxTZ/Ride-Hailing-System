/** Display formatting shared by booking screens. */

/** Whole-unit fares, e.g. 4000 → "4,000" (rounding policy per currency lands in the pricing engine). */
export function formatMoney(v?: string | number | null): string {
  return Math.round(Number(v ?? 0)).toLocaleString("en-US");
}

/** Clock time a number of seconds from now, e.g. "1:10 pm". */
export function clockIn(seconds: number, now = new Date()): string {
  const t = new Date(now.getTime() + seconds * 1000);
  const h = t.getHours();
  const m = String(t.getMinutes()).padStart(2, "0");
  return `${h % 12 === 0 ? 12 : h % 12}:${m} ${h < 12 ? "am" : "pm"}`;
}
