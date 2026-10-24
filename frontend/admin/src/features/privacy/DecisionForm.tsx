"use client";
/**
 * DecisionForm — approve or reject a privacy request with a reason (kept with
 * the request and in the audit log; shown to the person when rejected). A
 * deletion states plainly what goes and what stays before staff approve it.
 */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { ErrorNote } from "@/components/ui/States";
import { useMe } from "@/features/me";
import { ApiError } from "@/lib/api";
import { dateTime } from "@/lib/format";
import { type PrivacyRequest, useDecide } from "./hooks";
import { STATUS_LABEL } from "./labels";

const REASON_MAX = 1000;

const DELETE_SCOPE =
  "This removes their name, phone, places and contacts. Trip and payment records are kept as the law requires.";
const EXPORT_SCOPE =
  "Approving builds a file of their data. They download it from the app; the link stops working after 7 days.";

export function DecisionForm({ request }: { request: PrivacyRequest }) {
  if (request.status === "REQUESTED") return <Form request={request} />;
  return <Handled request={request} />;
}

function Handled({ request: r }: { request: PrivacyRequest }) {
  const by = r.handled_by ?? (r.status === "CANCELLED" ? "them" : "staff");
  return (
    <div className="grid gap-2 text-xs">
      {r.status === "IN_PROGRESS" ? (
        <p>The file is being built. It is usually ready within a few minutes.</p>
      ) : null}
      <p>{`${STATUS_LABEL[r.status]}${r.handled_at ? `, ${by}, ${dateTime(r.handled_at)}` : ""}.`}</p>
      {r.reason ? <p className="rounded-[8px] bg-surface-2 px-3 py-2 leading-relaxed">{r.reason}</p> : null}
    </div>
  );
}

function Form({ request: r }: { request: PrivacyRequest }) {
  const decide = useDecide(r.id);
  const [reason, setReason] = useState("");
  const canHandle = useMe().can("privacy.handle");
  if (!canHandle)
    return (
      <p className="text-xs text-muted">
        You can see this request, but handling it needs the "Handle privacy requests" permission.
      </p>
    );
  const isDelete = r.kind === "DELETE";
  const blocked = isDelete && r.blockers.length > 0;
  const send = (decision: "approve" | "reject") =>
    reason.trim() && decide.mutate({ decision, reason: reason.trim() });
  return (
    <div className="grid gap-3">
      <p
        className={`rounded-[8px] px-3 py-2 text-xs leading-relaxed ${isDelete ? "bg-[color-mix(in_srgb,var(--critical)_10%,transparent)] text-critical" : "bg-surface-2"}`}
      >
        {isDelete ? DELETE_SCOPE : EXPORT_SCOPE}
      </p>
      <Field label="Reason" hint="Kept with the request and in the audit log. Shown to them if you reject.">
        <textarea
          value={reason}
          maxLength={REASON_MAX}
          rows={4}
          onChange={(e) => setReason(e.target.value)}
          className="w-full rounded-[8px] bg-surface-2 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand"
        />
      </Field>
      <div className="flex flex-wrap gap-2">
        <Button
          variant={isDelete ? "danger" : "primary"}
          disabled={!reason.trim() || blocked}
          loading={decide.isPending && decide.variables?.decision === "approve"}
          onClick={() => send("approve")}
        >
          {isDelete ? "Delete account" : "Build their file"}
        </Button>
        <Button
          variant="soft"
          disabled={!reason.trim()}
          loading={decide.isPending && decide.variables?.decision === "reject"}
          onClick={() => send("reject")}
        >
          Reject
        </Button>
      </div>
      {blocked ? <p className="text-[11px] text-muted">Settle what is blocking the deletion first.</p> : null}
      {decide.error ? <DecideError error={decide.error} /> : null}
    </div>
  );
}

/** DELETE_BLOCKED gets plain words (the blockers card has refreshed); anything else as is. */
function DecideError({ error }: { error: Error }) {
  if (error instanceof ApiError && error.code === "DELETE_BLOCKED")
    return (
      <p className="text-xs text-critical">
        Something still blocks this deletion. The list on this page is up to date; settle it and try again.
      </p>
    );
  return <ErrorNote error={error} />;
}
