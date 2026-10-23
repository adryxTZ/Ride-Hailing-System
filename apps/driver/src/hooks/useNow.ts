/** useNow — the current time, re-rendering every `intervalMs` while `active` (countdowns). */
import { useEffect, useState } from "react";

const DEFAULT_TICK_MS = 1000;

export function useNow(active = true, intervalMs = DEFAULT_TICK_MS): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [active, intervalMs]);
  return now;
}
