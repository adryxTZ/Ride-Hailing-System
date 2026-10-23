/** useOnlineTimer — elapsed time since the driver went online, ticking every second while online. */
import { useEffect, useState } from "react";

const TICK_MS = 1000;

export function useOnlineTimer(online: boolean): string {
  const [since, setSince] = useState<number>();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    setSince(online ? Date.now() : undefined);
    setNow(Date.now());
    if (!online) return;
    const id = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(id);
  }, [online]);
  return since ? formatElapsed(now - since) : "00:00";
}

/** 65_000 → "01:05"; 3_725_000 → "1:02:05". */
export function formatElapsed(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mmss = `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  return h > 0 ? `${h}:${mmss}` : mmss;
}
