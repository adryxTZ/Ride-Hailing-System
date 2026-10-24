"use client";
/**
 * WorkflowsScreen — Settings › Approvals: every action that can need approval,
 * whether its workflow is on, when it applies and how many steps it has.
 * A row opens the action's editor page. Everything is off until turned on.
 */
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import { when } from "@/lib/format";
import { type ActionWorkflow, type Definition, useWorkflows } from "./hooks";

/** "Amount ≥ 100,000 · TZ" or "Always". */
export function conditionText(d: Definition): string {
  const c = d.conditions;
  const parts: string[] = [];
  const n = (v: string) => Number(v).toLocaleString("en-US");
  if (c.amount_min && c.amount_max) parts.push(`Amount ${n(c.amount_min)} to ${n(c.amount_max)}`);
  else if (c.amount_min) parts.push(`Amount ≥ ${n(c.amount_min)}`);
  else if (c.amount_max) parts.push(`Amount ≤ ${n(c.amount_max)}`);
  if (c.countries?.length) parts.push(c.countries.join(", "));
  return parts.length ? parts.join(" · ") : "Always";
}

export function WorkflowsScreen() {
  const { data, isLoading, error } = useWorkflows();
  const canEdit = useMe().can("approvals.manage");
  return (
    <div className="grid gap-4">
      <PageHeader
        title="Approvals"
        subtitle="Choose which changes need someone else's approval, when, and by whom. A change held for approval waits in the Approvals inbox."
      />
      {!canEdit ? (
        <ErrorNote error={new Error('Editing workflows needs the "Edit approval workflows" permission.')} />
      ) : null}
      <Card index={1}>
        {error ? <ErrorNote error={error} /> : null}
        {isLoading ? (
          <Skeleton className="h-64" />
        ) : !data?.data.length ? (
          <Empty title="No actions" body="Approvable actions appear here when their modules are on." />
        ) : (
          <WorkflowTable rows={data.data} />
        )}
      </Card>
    </div>
  );
}

function WorkflowTable({ rows }: { rows: ActionWorkflow[] }) {
  const router = useRouter();
  return (
    <Table head={["Action", "Area", "Workflow", "Applies when", "Steps", "Version"]}>
      {rows.map(({ action, workflow }) => (
        <Row
          key={action.key}
          onClick={() => router.push(`/settings/approvals/${encodeURIComponent(action.key)}`)}
        >
          <Cell className="text-xs">
            <span className="block font-semibold">{action.label}</span>
            <span className="text-[11px] text-muted">{action.description}</span>
          </Cell>
          <Cell className="text-xs text-muted">{action.group}</Cell>
          <Cell>{workflow.enabled ? <Chip tone="ok">On</Chip> : <Chip tone="muted">Off</Chip>}</Cell>
          <Cell className="text-xs">{workflow.enabled ? conditionText(workflow.definition) : "—"}</Cell>
          <Cell className="tnum text-xs">{workflow.enabled ? workflow.definition.steps.length : "—"}</Cell>
          <Cell className="tnum whitespace-nowrap text-[11px] text-muted">
            {workflow.version ? `v${workflow.version} · ${when(workflow.updated_at)}` : "Never set"}
          </Cell>
        </Row>
      ))}
    </Table>
  );
}
