/**
 * approval — the one place gated actions handle "202 Sent for approval". A
 * gated endpoint answers 202 {approval_request} when its approval workflow
 * holds the change instead of applying it. unwrapGated turns that into null
 * (the change did not happen yet) and announces the request, so the shared
 * ApprovalNotice tells the person and links to it; any other answer unwraps
 * as before.
 */
import { type Schemas, unwrap } from "@/lib/api";

export type ApprovalRequest = Schemas["ApprovalRequest"];
type Accepted = Schemas["ApprovalAccepted"];

/** HTTP status a gated endpoint uses when the change waits for approval. */
const SENT_FOR_APPROVAL = 202;

type Listener = (r: ApprovalRequest) => void;
const listeners = new Set<Listener>();

/** Subscribes to "a change was sent for approval"; returns the unsubscribe. */
export function onSentForApproval(fn: Listener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

function announce(r: ApprovalRequest) {
  for (const fn of listeners) fn(r);
}

function isAccepted(v: unknown): v is Accepted {
  return typeof v === "object" && v !== null && "approval_request" in v;
}

/**
 * Unwraps a gated call: its data, or null when the change was sent for
 * approval (the notice shows it). Screens treat null as "done for now".
 */
export function unwrapGated<T>(r: {
  data?: T;
  error?: unknown;
  response: Response;
}): Exclude<T, Accepted> | null {
  if (r.response.status === SENT_FOR_APPROVAL && isAccepted(r.data)) {
    announce(r.data.approval_request);
    return null;
  }
  return unwrap(r) as Exclude<T, Accepted>;
}
