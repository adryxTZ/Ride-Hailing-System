/**
 * StepProgress — the workflow a request runs under (the version it started
 * with), each step's rule and who approved it; the current step is marked.
 */
import { Check, Circle, X } from "lucide-react";
import { Chip } from "@/components/ui/Chip";
import { dateTime } from "@/lib/format";
import type { ApprovalRequest, Step } from "./hooks";
import { MODE_LABEL, memberLabel } from "./labels";

/** What a step needs, in words: "2 approvals · Finance or Operations". */
export function stepRule(s: Step): string {
  const members = [...s.roles, ...(s.permission ? [`perm:${s.permission}`] : [])].map(memberLabel);
  if (s.mode === "ALL") return `${MODE_LABEL.ALL}: ${members.join(", ")}`;
  const need = s.approvals === 1 ? "1 approval" : `${s.approvals} approvals`;
  return `${need} · ${members.join(" or ")}`;
}

export function StepProgress({ request: r }: { request: ApprovalRequest }) {
  return (
    <ol className="grid gap-3">
      {r.workflow.steps.map((s, i) => {
        const decisions = r.decisions.filter((d) => d.step === i);
        const state =
          i < r.current_step || r.status === "EXECUTED" ? "done" : i === r.current_step ? "now" : "later";
        return (
          <li key={`${s.name}${i}`} className="grid gap-2 rounded-[8px] border border-line px-3 py-2.5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="flex items-center gap-2 text-xs font-semibold">
                <StepIcon state={state} rejected={decisions.some((d) => d.decision === "REJECT")} />
                {`${i + 1}. ${s.name}`}
              </span>
              {state === "now" && r.status === "PENDING" ? <Chip tone="warn">Now</Chip> : null}
            </div>
            <p className="text-[11px] text-muted">{stepRule(s)}</p>
            {decisions.length ? (
              <ul className="grid gap-1">
                {decisions.map((d) => (
                  <li key={d.approver.id} className="text-[11px]">
                    <span
                      className={d.decision === "REJECT" ? "font-semibold text-critical" : "font-semibold"}
                    >
                      {d.decision === "REJECT" ? "Rejected" : "Approved"}
                    </span>
                    {` by ${d.approver.name}`}
                    {d.as_role ? <span className="text-muted">{` as ${memberLabel(d.as_role)}`}</span> : null}
                    <span className="tnum text-muted">{` · ${dateTime(d.at)}`}</span>
                    {d.note ? <span className="block text-muted">{`“${d.note}”`}</span> : null}
                  </li>
                ))}
              </ul>
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}

function StepIcon({ state, rejected }: { state: "done" | "now" | "later"; rejected: boolean }) {
  if (rejected) return <X size={14} className="text-critical" aria-label="Rejected" />;
  if (state === "done") return <Check size={14} className="text-good" aria-label="Done" />;
  return <Circle size={12} className={state === "now" ? "text-warn" : "text-muted"} aria-hidden="true" />;
}
