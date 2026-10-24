/** Where a privacy request's person and blockers are handled in the dashboard. */
import type { Blocker, PrivacyRequest } from "./hooks";
import { BLOCKER_PLACE } from "./labels";

/** True once a deletion has run: the account is anonymised and has no page or name. */
export function isAnonymised(r: PrivacyRequest): boolean {
  return (r.kind === "DELETE" && r.status === "DONE") || !!r.user_phone?.startsWith("deleted:");
}

/** The name staff see: their name, else their phone, else "Deleted account". */
export function personName(r: PrivacyRequest): string {
  if (isAnonymised(r)) return "Deleted account";
  return r.user_name || r.user_phone || "Unknown";
}

/** The person's own page, by the app they asked from (none once anonymised). */
export function personHref(r: PrivacyRequest): string | undefined {
  if (!r.user_id || isAnonymised(r)) return undefined;
  return r.app === "DRIVER" ? `/drivers/${r.user_id}` : `/riders/${r.user_id}`;
}

/** The page where staff settle a blocker, with its link label. */
export function blockerLink(r: PrivacyRequest, b: Blocker): { href: string; label: string } | undefined {
  const place = BLOCKER_PLACE[b];
  if (place === "finance") return { href: "/finance", label: "Open Finance" };
  if (place === "trips") return { href: "/trips?status=active", label: "Open active trips" };
  const href = personHref(r);
  return href ? { href, label: r.app === "DRIVER" ? "Open driver page" : "Open rider page" } : undefined;
}
