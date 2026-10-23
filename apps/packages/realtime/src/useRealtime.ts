/** useRealtime — keeps one socket open while `enabled`; the latest handler always receives frames. */
import { useEffect, useRef, useState } from "react";
import { createRealtime, type RealtimeFrame, type RealtimeStatus } from "./client";

export function useRealtime(opts: {
  url: string;
  enabled: boolean;
  getToken: () => Promise<string | undefined>;
  onFrame: (frame: RealtimeFrame) => void;
}): RealtimeStatus {
  const [status, setStatus] = useState<RealtimeStatus>("closed");
  const handler = useRef(opts.onFrame);
  handler.current = opts.onFrame;
  const tokenFn = useRef(opts.getToken);
  tokenFn.current = opts.getToken;

  useEffect(() => {
    if (!opts.enabled) return;
    const rt = createRealtime({
      url: opts.url,
      getToken: () => tokenFn.current(),
      onFrame: (f) => handler.current(f),
      onStatus: setStatus,
    });
    return () => rt.stop();
  }, [opts.enabled, opts.url]);

  return status;
}
