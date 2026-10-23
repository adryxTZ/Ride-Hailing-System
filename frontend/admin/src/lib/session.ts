/**
 * session — the staff member's tokens in localStorage, and a single-flight
 * refresh: when the access token expires, one refresh runs and every waiting
 * request reuses its result (two parallel refreshes would revoke the session).
 */
import { API_BASE_URL } from "@/lib/config";

const ACCESS = "platform.admin.access";
const REFRESH = "platform.admin.refresh";

export interface Tokens {
  access_token: string;
  refresh_token: string;
}

const read = (k: string) => (typeof window === "undefined" ? null : window.localStorage.getItem(k));

export function getAccess(): string | null {
  return read(ACCESS);
}

export function saveTokens(t: Tokens) {
  window.localStorage.setItem(ACCESS, t.access_token);
  window.localStorage.setItem(REFRESH, t.refresh_token);
  window.dispatchEvent(new Event("platform-session"));
}

export function clearSession() {
  window.localStorage.removeItem(ACCESS);
  window.localStorage.removeItem(REFRESH);
  window.dispatchEvent(new Event("platform-session"));
}

let inFlight: Promise<string | null> | null = null;

/** Refreshes the access token once for all callers; signs out when the refresh is refused. */
export function refreshAccess(): Promise<string | null> {
  if (inFlight) return inFlight;
  const refresh = read(REFRESH);
  if (!refresh) return Promise.resolve(null);
  inFlight = fetch(`${API_BASE_URL}/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refresh, app: "admin" }),
  })
    .then(async (r) => {
      if (!r.ok) {
        clearSession();
        return null;
      }
      const t = (await r.json()) as Tokens;
      saveTokens(t);
      return t.access_token;
    })
    .catch(() => null)
    .finally(() => {
      inFlight = null;
    });
  return inFlight;
}

/** Claims of the current access token (roles, subject), decoded without verifying. */
export function claims(): { sub?: string; roles?: string[] } {
  const t = getAccess();
  if (!t) return {};
  try {
    return JSON.parse(atob(t.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));
  } catch {
    return {};
  }
}

/** Staff roles from the token ("Super Admin", "Finance" …). */
export function staffRoles(): string[] {
  return (claims().roles ?? []).filter((r) => r.startsWith("staff:")).map((r) => r.slice("staff:".length));
}

/** Permissions from the token ("pricing.edit" …; "*" = all). */
export function permissions(): string[] {
  return (claims().roles ?? []).filter((r) => r.startsWith("perm:")).map((r) => r.slice("perm:".length));
}

/** Countries and regions the person is limited to; empty = everywhere. */
export function accessScopes(): { country: string; areaId?: string }[] {
  return (claims().roles ?? [])
    .filter((r) => r.startsWith("scope:"))
    .map((r) => {
      const [country, areaId] = r.slice("scope:".length).split("/");
      return { country, areaId: areaId || undefined };
    });
}
