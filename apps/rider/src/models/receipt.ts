/** receipt — the fare lines of a trip: what was quoted plus what was added during the ride (waiting). */
import type { FareLineView } from "@/components/taxi/FareBreakdown";
import type { Trip } from "@/services/trips";

export function receiptLines(trip?: Trip | null): FareLineView[] {
  const quoted = ((trip?.quote_option as { lines?: FareLineView[] } | undefined)?.lines ??
    []) as FareLineView[];
  const added = (trip?.adjustments ?? []) as FareLineView[];
  return [...quoted, ...added];
}
