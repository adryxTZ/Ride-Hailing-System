"use client";
/**
 * WorkflowEditorScreen — one action's approval workflow: on or off, when it
 * applies (amount, countries), how long a request may wait, and its ordered
 * steps. Saving asks why and makes a new version; requests already waiting
 * keep the version they started under. The history lists every version.
 */
import { ArrowLeft, Plus } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Toggle } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import { ApiError, type Schemas } from "@/lib/api";
import { dateTime } from "@/lib/format";
import { useCountries } from "@/lib/scope";
import { type Definition, type Step, useSaveWorkflow, useWorkflow, useWorkflows } from "./hooks";
import { StepEditor } from "./StepEditor";
import { stepRule } from "./StepProgress";
import { conditionText } from "./WorkflowsScreen";

interface Draft {
  enabled: boolean;
  definition: Definition;
}

const NEW_STEP: Step = { name: "", mode: "ANY", roles: [], approvals: 1 };

export function WorkflowEditorScreen({ action }: { action: string }) {
  const { data, isLoading, error } = useWorkflow(action);
  const [draft, setDraft] = useState<Draft>();
  useEffect(() => {
    if (data && !draft)
      setDraft({ enabled: data.workflow.enabled, definition: structuredClone(data.workflow.definition) });
  }, [data, draft]);
  if (isLoading || (data && !draft)) return <Skeleton className="h-96" />;
  if (error || !data || !draft) return <ErrorNote error={error ?? new Error("Action not found")} />;
  return (
    <div>
      <PageHeader
        title={data.action.label}
        subtitle={data.action.description}
        actions={
          <Link
            href="/settings/approvals"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
          >
            <ArrowLeft size={14} aria-hidden="true" /> All workflows
          </Link>
        }
      />
      <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <Editor action={data.action} draft={draft} onChange={setDraft} onSaved={(d) => setDraft(d)} />
        <History history={data.history} />
      </div>
    </div>
  );
}

function Editor({
  action,
  draft,
  onChange,
  onSaved,
}: {
  action: Schemas["ApprovalAction"];
  draft: Draft;
  onChange: (d: Draft) => void;
  onSaved: (d: Draft) => void;
}) {
  const { data: opts } = useWorkflows();
  const save = useSaveWorkflow(action.key);
  const canEdit = useMe().can("approvals.manage");
  const [asking, setAsking] = useState(false);
  const d = draft.definition;
  const setDef = (patch: Partial<Definition>) => onChange({ ...draft, definition: { ...d, ...patch } });
  const setStep = (i: number, s: Step) => setDef({ steps: d.steps.map((x, j) => (j === i ? s : x)) });
  const move = (i: number, by: -1 | 1) => {
    const steps = [...d.steps];
    [steps[i], steps[i + by]] = [steps[i + by], steps[i]];
    setDef({ steps });
  };
  return (
    <div className="grid gap-4">
      <Card index={1} title="Workflow">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="grid gap-0.5 text-sm">
            <span className="font-medium">
              {draft.enabled ? "On: matching changes wait for approval" : "Off: changes apply at once"}
            </span>
            <span className="text-xs text-muted">{`Asking for it needs the ${action.permission} permission.`}</span>
          </div>
          <Toggle
            label="Workflow on"
            checked={draft.enabled}
            onChange={(enabled) => onChange({ ...draft, enabled })}
          />
        </div>
      </Card>
      <ConditionsCard action={action} definition={d} onChange={setDef} />
      <Card
        index={3}
        title="Steps"
        action={
          <Button
            size="sm"
            variant="soft"
            icon={<Plus size={14} aria-hidden="true" />}
            onClick={() => setDef({ steps: [...d.steps, { ...NEW_STEP, roles: [] }] })}
          >
            Add step
          </Button>
        }
      >
        {d.steps.length ? (
          <ol className="grid gap-3">
            {d.steps.map((s, i) => (
              <StepEditor
                key={i}
                index={i}
                count={d.steps.length}
                step={s}
                roles={opts?.roles ?? []}
                permissions={opts?.permissions ?? []}
                onChange={(v) => setStep(i, v)}
                onMove={(by) => move(i, by)}
                onRemove={() => setDef({ steps: d.steps.filter((_, j) => j !== i) })}
              />
            ))}
          </ol>
        ) : (
          <Empty
            title="No steps yet"
            body="Add who must approve. Steps run in order; the requester never approves their own change."
          />
        )}
      </Card>
      <SaveProblems error={save.error} />
      <div className="flex justify-end">
        <Button disabled={!canEdit} onClick={() => setAsking(true)}>
          Save workflow
        </Button>
      </div>
      <ReasonDialog
        open={asking}
        title="Save this workflow?"
        body="It becomes a new version. Requests already waiting keep the version they started under."
        confirm="Save"
        busy={save.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) =>
          save.mutate(
            { enabled: draft.enabled, definition: d, reason },
            {
              onSuccess: (w) => {
                setAsking(false);
                onSaved({ enabled: w.workflow.enabled, definition: structuredClone(w.workflow.definition) });
              },
              onError: () => setAsking(false),
            },
          )
        }
      />
    </div>
  );
}

function ConditionsCard({
  action,
  definition: d,
  onChange,
}: {
  action: Schemas["ApprovalAction"];
  definition: Definition;
  onChange: (patch: Partial<Definition>) => void;
}) {
  const { data: countries } = useCountries();
  const c = d.conditions;
  const chosen = c.countries ?? [];
  const toggle = (code: string) =>
    onChange({
      conditions: {
        ...c,
        countries: chosen.includes(code) ? chosen.filter((x) => x !== code) : [...chosen, code],
      },
    });
  const amount = (key: "amount_min" | "amount_max", v: string) =>
    onChange({ conditions: { ...c, [key]: v.trim() === "" ? undefined : v.trim() } });
  return (
    <Card
      index={2}
      title="Applies when"
      action={<span className="text-xs text-muted">{conditionText(d)}</span>}
    >
      <div className="grid gap-4">
        {action.condition === "amount" ? (
          <div className="grid gap-3 md:grid-cols-2">
            <Field label="Amount at least" hint="Leave empty for any amount. In the request's own currency.">
              <Input
                inputMode="decimal"
                value={c.amount_min ?? ""}
                onChange={(e) => amount("amount_min", e.target.value)}
              />
            </Field>
            <Field label="Amount at most" hint="Optional upper limit.">
              <Input
                inputMode="decimal"
                value={c.amount_max ?? ""}
                onChange={(e) => amount("amount_max", e.target.value)}
              />
            </Field>
          </div>
        ) : null}
        {action.scoped ? (
          <div className="grid gap-1.5 text-sm">
            <span className="font-medium">Countries</span>
            <div className="flex flex-wrap gap-2" role="group" aria-label="Countries">
              {(countries ?? []).map((k) => (
                <button
                  key={k.code}
                  type="button"
                  aria-pressed={chosen.includes(k.code)}
                  onClick={() => toggle(k.code)}
                  className={`rounded-[8px] px-3 py-1.5 text-xs font-medium transition-colors ${chosen.includes(k.code) ? "bg-nav text-nav-ink" : "bg-surface-2 text-muted hover:text-ink"}`}
                >
                  {k.name?.en ?? k.code}
                </button>
              ))}
            </div>
            <span className="text-xs text-muted">None chosen means every country.</span>
          </div>
        ) : (
          <p className="text-xs text-muted">
            This change is not tied to a country, so it applies everywhere.
          </p>
        )}
        <Field
          label="Expires after (hours)"
          hint="A request nobody decides in time expires and nothing changes. 1 to 720."
        >
          <Input
            type="number"
            min={1}
            max={720}
            value={d.expire_hours}
            onChange={(e) => onChange({ expire_hours: Number(e.target.value) || 0 })}
            className="max-w-40"
          />
        </Field>
      </div>
    </Card>
  );
}

/** The server's list of what is wrong with the workflow, one per line. */
function SaveProblems({ error }: { error: Error | null }) {
  if (!error) return null;
  const problems = error instanceof ApiError ? (error.details?.problems as string[] | undefined) : undefined;
  if (!problems?.length) return <ErrorNote error={error} />;
  return (
    <div
      role="alert"
      className="grid gap-1 rounded-[10px] bg-[color-mix(in_srgb,var(--critical)_10%,transparent)] px-4 py-3 text-sm text-critical"
    >
      <span className="font-semibold">Fix these and save again:</span>
      <ul className="list-disc pl-5 text-xs">
        {problems.map((p) => (
          <li key={p}>{p}</li>
        ))}
      </ul>
    </div>
  );
}

function History({ history }: { history: Schemas["ApprovalWorkflowVersion"][] }) {
  return (
    <Card index={4} title="Versions">
      {!history.length ? (
        <Empty title="Never saved" body="Saving the workflow records version 1 here." />
      ) : (
        <ul className="divide-y divide-line">
          {history.map((v) => (
            <li key={v.version} className="grid gap-1 py-3 text-[11px] first:pt-0 last:pb-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="tnum text-xs font-semibold">{`v${v.version}`}</span>
                {v.enabled ? <Chip tone="ok">On</Chip> : <Chip tone="muted">Off</Chip>}
                <span className="tnum text-muted">{`${dateTime(v.created_at)} · ${v.by_name ?? "Unknown"}`}</span>
              </div>
              <span className="text-muted">{`Applies: ${conditionText(v.definition)}`}</span>
              {v.definition.steps.map((s, i) => (
                <span
                  key={`${s.name}${i}`}
                  className="text-muted"
                >{`${i + 1}. ${s.name}: ${stepRule(s)}`}</span>
              ))}
              <span className="leading-relaxed">{`Why: ${v.reason}`}</span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
