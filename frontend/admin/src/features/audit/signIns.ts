/** Sign-in entries from the audit log, shaped for the map and the table. */
import { when } from "@/lib/format";

export interface SignIn {
  id: number;
  who: string;
  when: string;
  at: string;
  failed: boolean;
  device: string;
  ip: string;
  userAgent: string;
  location?: { lat: number; lng: number; accuracy_m?: number };
}

type Entry = {
  id: number;
  actor?: string | null;
  action: string;
  at: string;
  data?: Record<string, unknown>;
};

export function toSignIn(e: Entry): SignIn {
  const d = e.data ?? {};
  const loc = d.location as SignIn["location"] | undefined;
  return {
    id: e.id,
    who: e.actor ?? "Unknown account",
    when: when(e.at),
    at: e.at,
    failed: e.action === "staff.login_failed",
    device: typeof d.device === "string" ? d.device : "Unknown device",
    ip: typeof d.ip === "string" ? d.ip : "",
    userAgent: typeof d.user_agent === "string" ? d.user_agent : "",
    location: loc && typeof loc.lat === "number" && typeof loc.lng === "number" ? loc : undefined,
  };
}

/** "−6.8161, 39.2894 (±25 m)" for the details panel. */
export function placeText(l?: SignIn["location"]): string {
  if (!l) return "Not shared";
  const acc = l.accuracy_m ? ` (±${Math.round(l.accuracy_m)} m)` : "";
  return `${l.lat.toFixed(4)}, ${l.lng.toFixed(4)}${acc}`;
}
