"use client";
/**
 * DecisionPanel — approve or reject the current step with a note; the
 * requester can only withdraw their own request. A finished request shows its
 * outcome: applied, or why the approved change did not apply.
 */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { ErrorNote } from "@/components/ui/States";
import { dateTime } from "@/lib/format";
import { claims } from "@/lib/session";
import { type ApprovalRequest, useDecideApproval } from "./hooks";
import { FAILURE_LABEL, STATUS_LABEL } from "./labels";

const NOTE_MAX = 1000;

export function DecisionPanel({ request: r }: { request: ApprovalRequest }) {
  if (r.status !== "PENDING") return <Outcome request={r} />;
  const mine = claims().sub === r.requester.id;
  if (mine) return <Withdraw request={r} />;
  if (!r.can_decide)
    return (
      <p className="text-xs leading-relaxed text-muted">
        This step needs someone holding one of the roles listed under Steps. You can follow it here.
      </p>
    );
  return <Decide request={r} />;
}

function NoteField({
  value,
  onChange,
  hint,
}: {
  value: string;
  onChange: (v: string) => void;
  hint: string;
}) {
  return (
    <Field label="Note" hint={hint}>
      <textarea
        value={value}
        maxLength={NOTE_MAX}
        rows={3}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-[8px] bg-surface-2 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand"
      />
    </Field>
  );
}

function Decide({ request: r }: { request: ApprovalRequest }) {
  const decide = useDecideApproval(r.id);
  const [note, setNote] = useState("");
  const last = r.current_step === r.workflow.steps.length - 1;
  return (
    <div className="grid gap-3">
      <p className="rounded-[8px] bg-surface-2 px-3 py-2 text-xs leading-relaxed">
        {last
          ? "If your approval completes this step, the change applies at once, recorded as asked by the requester and approved by you."
          : "Your approval counts toward this step; the next step's approvers decide after."}
      </p>
      <NoteField
        value={note}
        onChange={setNote}
        hint="Kept with the request and in the audit log. Needed to reject."
      />
      <div className="flex flex-wrap gap-2">
        <Button
          loading={decide.isPending && decide.variables?.decision === "approve"}
          onClick={() => decide.mutate({ decision: "approve", note: note.trim() })}
        >
          Approve
        </Button>
        <Button
          variant="danger"
          disabled={!note.trim()}
          loading={decide.isPending && decide.variables?.decision === "reject"}
          onClick={() => decide.mutate({ decision: "reject", note: note.trim() })}
        >
          Reject
        </Button>
      </div>
      <ErrorNote error={decide.error} />
    </div>
  );
}

function Withdraw({ request: r }: { request: ApprovalRequest }) {
  const decide = useDecideApproval(r.id);
  const [note, setNote] = useState("");
  return (
    <div className="grid gap-3">
      <p className="text-xs leading-relaxed text-muted">
        You asked for this change, so someone else approves it. You can withdraw it while it waits.
      </p>
      <NoteField value={note} onChange={setNote} hint="Optional. Kept in the audit log." />
      <div>
        <Button
          variant="soft"
          loading={decide.isPending}
          onClick={() => decide.mutate({ decision: "cancel", note: note.trim() })}
        >
          Withdraw request
        </Button>
      </div>
      <ErrorNote error={decide.error} />
    </div>
  );
}

function Outcome({ request: r }: { request: ApprovalRequest }) {
  const at = r.executed_at ?? r.decided_at;
  return (
    <div className="grid gap-2 text-xs">
      <p className="font-semibold">{`${STATUS_LABEL[r.status]}${at ? `, ${dateTime(at)}` : ""}.`}</p>
      {r.status === "FAILED" && r.error ? (
        <p className="rounded-[8px] bg-[color-mix(in_srgb,var(--critical)_10%,transparent)] px-3 py-2 leading-relaxed text-critical">
          {FAILURE_LABEL[r.error] ?? `It was approved but could not be applied (${r.error}).`}
        </p>
      ) : null}
      {r.status === "EXECUTED" ? (
        <p className="text-muted">The change was applied as asked; the audit log names who approved it.</p>
      ) : null}
      {r.status === "EXPIRED" ? (
        <p className="text-muted">Nobody decided in time. Make the change again to ask anew.</p>
      ) : null}
    </div>
  );
}
