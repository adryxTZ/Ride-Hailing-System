/**
 * RealtimeClient — one socket to /v1/ws for the signed-in user.
 *
 *  - auth: the first frame carries the access token (never the URL);
 *  - heartbeat: {"type":"ping"} every HEARTBEAT_MS; a socket silent for
 *    SILENCE_MS is treated as dead and replaced;
 *  - reconnect: exponential backoff with jitter, reset after a healthy session;
 *  - the token is fetched fresh for every attempt (the app refreshes it if needed).
 * Frames are delivered to `onFrame`; callers re-read state over HTTP on `open`
 * because the server does not replay missed frames.
 */

export interface RealtimeFrame<T = unknown> {
  type: string;
  ts: string;
  data?: T;
}

export type RealtimeStatus = "connecting" | "open" | "closed";

export interface RealtimeOptions {
  url: string;
  getToken: () => Promise<string | undefined>;
  onFrame: (frame: RealtimeFrame) => void;
  onStatus?: (status: RealtimeStatus) => void;
}

const HEARTBEAT_MS = 20_000;
const SILENCE_MS = 50_000;
const BACKOFF_START_MS = 1_000;
const BACKOFF_MAX_MS = 15_000;

export function createRealtime(opts: RealtimeOptions): { stop: () => void } {
  let ws: WebSocket | undefined;
  let stopped = false;
  let backoff = BACKOFF_START_MS;
  let heartbeat: ReturnType<typeof setInterval> | undefined;
  let retry: ReturnType<typeof setTimeout> | undefined;
  let lastHeard = Date.now();

  const status = (s: RealtimeStatus) => opts.onStatus?.(s);

  const cleanup = () => {
    if (heartbeat) clearInterval(heartbeat);
    heartbeat = undefined;
    if (ws) {
      ws.onopen = ws.onmessage = ws.onclose = ws.onerror = null;
      try {
        ws.close();
      } catch {}
    }
    ws = undefined;
  };

  const scheduleReconnect = () => {
    cleanup();
    status("closed");
    if (stopped) return;
    const delay = backoff + Math.random() * 500;
    backoff = Math.min(backoff * 2, BACKOFF_MAX_MS);
    retry = setTimeout(connect, delay);
  };

  async function connect() {
    if (stopped) return;
    const token = await opts.getToken().catch(() => undefined);
    if (!token) return scheduleReconnect();
    status("connecting");
    const sock = new WebSocket(opts.url);
    ws = sock;
    sock.onopen = () => sock.send(JSON.stringify({ type: "auth", token }));
    sock.onmessage = (e) => {
      lastHeard = Date.now();
      let frame: RealtimeFrame;
      try {
        frame = JSON.parse(String(e.data)) as RealtimeFrame;
      } catch {
        return;
      }
      if (frame.type === "ready") {
        backoff = BACKOFF_START_MS;
        status("open");
        startHeartbeat(sock);
      }
      if (frame.type !== "pong") opts.onFrame(frame);
    };
    sock.onclose = scheduleReconnect;
    sock.onerror = () => sock.close();
  }

  function startHeartbeat(sock: WebSocket) {
    lastHeard = Date.now();
    heartbeat = setInterval(() => {
      if (Date.now() - lastHeard > SILENCE_MS) return scheduleReconnect();
      if (sock.readyState === WebSocket.OPEN) sock.send(JSON.stringify({ type: "ping" }));
    }, HEARTBEAT_MS);
  }

  void connect();
  return {
    stop() {
      stopped = true;
      if (retry) clearTimeout(retry);
      cleanup();
      status("closed");
    },
  };
}
