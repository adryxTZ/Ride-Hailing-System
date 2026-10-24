"use client";
/**
 * RequestScreen — one held change: what it does (field by field), who asked
 * and why, the steps of the workflow version it started under with who
 * approved, and the decision.
 */
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { dateTime } from "@/lib/format";
import { ChangeTable } from "./ChangeTable";
import { DecisionPanel } from "./DecisionPanel";
import { type ApprovalRequest, useApprovalRequest } from "./hooks";
import { STATUS_LABEL, STATUS_TONE } from "./labels";
import { StepProgress } from "./StepProgress";

export function RequestScreen({ id }: { id: string }) {
  const { data: r, isLoading, error } = useApprovalRequest(id);
  if (isLoading) return <Skeleton className="h-96" />;
  if (error || !r) return <ErrorNote error={error ?? new Error("Request not found")} />;
  return (
    <div>
      <PageHeader
        title={r.summary.title || r.action_label}
        subtitle={`${r.action_label}. Asked by ${r.requester.name || "someone"} ${dateTime(r.created_at)}.`}
        actions={
          <Link
            href="/approvals"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
          >
            <ArrowLeft size={14} aria-hidden="true" /> Approvals
          </Link>
        }
      />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <div className="grid content-start gap-4">
          <Card title="What changes" index={1}>
            <ChangeTable changes={r.summary.changes} />
          </Card>
          <Card title="Request" index={2}>
            <About r={r} />
          </Card>
        </div>
        <div className="grid content-start gap-4">
          <Card title="Decision" index={3}>
            <DecisionPanel request={r} />
          </Card>
          <Card title={`Steps (workflow version ${r.workflow_version})`} index={4}>
            <StepProgress request={r} />
          </Card>
        </div>
      </div>
    </div>
  );
}

function About({ r }: { r: ApprovalRequest }) {
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2.5 text-xs">
      <dt className="text-muted">Status</dt>
      <dd>
        <Chip tone={STATUS_TONE[r.status]}>{STATUS_LABEL[r.status]}</Chip>
      </dd>
      <dt className="text-muted">Asked by</dt>
      <dd className="font-semibold">{r.requester.name || "Unknown"}</dd>
      <dt className="text-muted">Why</dt>
      <dd className="leading-relaxed">{r.reason}</dd>
      <dt className="text-muted">Country</dt>
      <dd>{r.country ?? "Every country"}</dd>
      {r.amount != null ? (
        <>
          <dt className="text-muted">Amount</dt>
          <dd className="tnum">{Number(r.amount).toLocaleString("en-US")}</dd>
        </>
      ) : null}
      <dt className="text-muted">Record</dt>
      <dd className="font-mono text-[11px] text-muted">{r.target || "—"}</dd>
      <dt className="text-muted">{r.status === "PENDING" ? "Expires" : "Was due by"}</dt>
      <dd className="tnum">{dateTime(r.expires_at)}</dd>
    </dl>
  );
}
