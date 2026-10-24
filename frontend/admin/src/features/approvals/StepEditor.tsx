"use client";
/**
 * StepEditor — one approval step: its name, who may approve (roles and/or a
 * permission), "any of" or "one from each", how many approvals, and moving
 * or removing it.
 */
import { ArrowDown, ArrowUp, Trash2 } from "lucide-react";
import { Field, Input, Select } from "@/components/ui/Field";
import type { Schemas } from "@/lib/api";
import type { Step } from "./hooks";
import { MODE_LABEL } from "./labels";

const MAX_APPROVALS = 10;

interface Props {
  index: number;
  count: number;
  step: Step;
  roles: string[];
  permissions: Schemas["Permission"][];
  onChange: (s: Step) => void;
  onMove: (by: -1 | 1) => void;
  onRemove: () => void;
}

export function StepEditor({ index, count, step, roles, permissions, onChange, onMove, onRemove }: Props) {
  const toggle = (role: string) =>
    onChange({
      ...step,
      roles: step.roles.includes(role) ? step.roles.filter((r) => r !== role) : [...step.roles, role],
    });
  return (
    <li className="grid gap-3 rounded-[8px] border border-line p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-muted">{`Step ${index + 1}`}</span>
        <div className="flex gap-1">
          <IconButton label="Move up" disabled={index === 0} onClick={() => onMove(-1)}>
            <ArrowUp size={14} aria-hidden="true" />
          </IconButton>
          <IconButton label="Move down" disabled={index === count - 1} onClick={() => onMove(1)}>
            <ArrowDown size={14} aria-hidden="true" />
          </IconButton>
          <IconButton label="Remove step" onClick={onRemove}>
            <Trash2 size={14} aria-hidden="true" />
          </IconButton>
        </div>
      </div>
      <div className="grid gap-3 md:grid-cols-[2fr_2fr_1fr]">
        <Field label="Name">
          <Input
            value={step.name}
            maxLength={80}
            onChange={(e) => onChange({ ...step, name: e.target.value })}
          />
        </Field>
        <Field label="Who must approve">
          <Select
            value={step.mode}
            onChange={(e) => onChange({ ...step, mode: e.target.value as Step["mode"] })}
          >
            <option value="ANY">{MODE_LABEL.ANY}</option>
            <option value="ALL">{MODE_LABEL.ALL}</option>
          </Select>
        </Field>
        <Field label="Approvals" hint={step.mode === "ALL" ? "One per role" : undefined}>
          <Input
            type="number"
            min={1}
            max={MAX_APPROVALS}
            disabled={step.mode === "ALL"}
            value={step.approvals}
            onChange={(e) => onChange({ ...step, approvals: Math.max(1, Number(e.target.value) || 1) })}
          />
        </Field>
      </div>
      <div className="grid gap-1.5 text-sm">
        <span className="font-medium">Roles</span>
        <div className="flex flex-wrap gap-2" role="group" aria-label={`Roles for step ${index + 1}`}>
          {roles.map((r) => (
            <button
              key={r}
              type="button"
              aria-pressed={step.roles.includes(r)}
              onClick={() => toggle(r)}
              className={`rounded-[8px] px-3 py-1.5 text-xs font-medium transition-colors ${step.roles.includes(r) ? "bg-nav text-nav-ink" : "bg-surface-2 text-muted hover:text-ink"}`}
            >
              {r}
            </button>
          ))}
        </div>
      </div>
      <Field label="Or anyone with this permission" hint="Optional. Super Admins hold every permission.">
        <Select
          value={step.permission ?? ""}
          onChange={(e) => onChange({ ...step, permission: e.target.value || undefined })}
        >
          <option value="">No permission</option>
          {permissions.map((p) => (
            <option key={p.key} value={p.key}>
              {`${p.group}: ${p.label}`}
            </option>
          ))}
        </Select>
      </Field>
    </li>
  );
}

function IconButton({
  label,
  disabled,
  onClick,
  children,
}: {
  label: string;
  disabled?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className="grid size-8 place-items-center rounded-[8px] text-muted transition-colors hover:bg-surface-2 hover:text-ink disabled:opacity-30"
    >
      {children}
    </button>
  );
}
