"use client";
/**
 * InboxScreen — the Approvals inbox: changes held back by an approval
 * workflow. "Waiting" lists what is pending in the person's countries (theirs
 * to decide first), "Mine" what they asked for, "Done" what was decided.
 * A row opens the request's own page.
 */
import { useRouter } from "next/navigation";
import { Suspense } from "react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { FilterBar } from "@/components/ui/Operations";
import { PageHeader } from "@/components/ui/PageHeader";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { dateTime, money } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { type ApprovalRequest, type Box, useApprovalCount, useApprovalRequests } from "./hooks";
import { STATUS_LABEL, STATUS_TONE } from "./labels";

const BOXES: readonly { value: Box; label: string }[] = [
  { value: "pending", label: "Waiting" },
  { value: "mine", label: "Mine" },
  { value: "done", label: "Done" },
];

const EMPTY: Record<Box, { title: string; body: string }> = {
  pending: { title: "Nothing waiting", body: "Changes held for approval in your countries appear here." },
  mine: { title: "You asked for nothing yet", body: "A change you make that needs approval appears here." },
  done: { title: "Nothing decided yet", body: "Approved, rejected and expired requests appear here." },
};

export function InboxScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Inbox />
    </Suspense>
  );
}

function Inbox() {
  const url = useUrlState();
  const box = (url.get("box") ?? "pending") as Box;
  const { data, isLoading, error } = useApprovalRequests(box);
  const { data: actionable } = useApprovalCount();
  return (
    <div className="grid gap-4">
      <PageHeader
        title="Approvals"
        subtitle="Changes that need someone else's approval before they apply. Open one to see exactly what it changes."
      />
      <Card index={1}>
        <div className="mb-4">
          <FilterBar
            label="Show"
            value={box}
            options={BOXES.map((b) => (b.value === "pending" ? { ...b, count: actionable } : b))}
            onChange={(v) => url.set({ box: v === "pending" ? undefined : v })}
          />
        </div>
        {error ? <ErrorNote error={error} /> : null}
        {isLoading ? (
          <Skeleton className="h-64" />
        ) : !data?.length ? (
          <Empty title={EMPTY[box].title} body={EMPTY[box].body} />
        ) : (
          <RequestsTable rows={data} />
        )}
      </Card>
    </div>
  );
}

/** The amount in the request's currency when the summary names it, else plain. */
function amountText(r: ApprovalRequest): string {
  if (r.amount == null) return "—";
  const currency = /\b([A-Z]{3})\b/.exec(r.summary.title)?.[1];
  return currency ? money(r.amount, currency) : Number(r.amount).toLocaleString("en-US");
}

function RequestsTable({ rows }: { rows: ApprovalRequest[] }) {
  const router = useRouter();
  return (
    <Table head={["Change", "Asked by", "Country", "Amount", "Step", "Asked", "Status"]}>
      {rows.map((r) => (
        <Row key={r.id} onClick={() => router.push(`/approvals/${r.id}`)}>
          <Cell className="text-xs">
            <span className="block font-semibold">{r.summary.title || r.action_label}</span>
            <span className="text-[11px] text-muted">{r.action_label}</span>
          </Cell>
          <Cell className="text-xs">{r.requester.name || "Unknown"}</Cell>
          <Cell className="text-xs text-muted">{r.country ?? "All"}</Cell>
          <Cell className="tnum whitespace-nowrap text-xs">{amountText(r)}</Cell>
          <Cell className="text-xs text-muted">
            {r.status === "PENDING"
              ? `${Math.min(r.current_step + 1, r.workflow.steps.length)} of ${r.workflow.steps.length}`
              : "—"}
          </Cell>
          <Cell className="tnum whitespace-nowrap text-[11px] text-muted">{dateTime(r.created_at)}</Cell>
          <Cell>
            <div className="flex flex-wrap gap-1.5">
              <Chip tone={STATUS_TONE[r.status]}>{STATUS_LABEL[r.status]}</Chip>
              {r.can_decide ? <Chip tone="live">Your turn</Chip> : null}
            </div>
          </Cell>
        </Row>
      ))}
    </Table>
  );
}
