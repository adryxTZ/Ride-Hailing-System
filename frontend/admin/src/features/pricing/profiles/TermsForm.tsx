"use client";
/**
 * TermsForm — a profile's money rules, grouped. On a region's profile every
 * trip rule has "Use default" (null = inherit, shown with the default's value)
 * and the tax, debt and cash-out rules are read-only: they come from the default.
 */
import { useState } from "react";
import { Field, Input, Select } from "@/components/ui/Field";
import {
  fractionToPct,
  GROUP_TITLE,
  pctToFraction,
  showTerm,
  TERM_FIELDS,
  type TermField,
  type Terms,
} from "./model";

const GROUPS = ["fees", "time", "tax", "debt", "cashout"] as const;
const DESCRIPTION = {
  fees: "Set the platform’s commission, booking fee and fare rounding.",
  time: "Define waiting limits, cancellation windows and the driver’s share.",
  tax: "Country rules for VAT and the transport levy.",
  debt: "Set debt limits, grace periods and the smallest driver payment.",
  cashout: "Control the minimum withdrawal and approval requirements.",
};
/** The unit staff type in, after the label. */
const UNIT: Partial<Record<TermField["kind"], string>> = { pct: " (%)", minutes: " (min)" };

export function TermsForm({
  terms,
  defaults,
  editable,
  currency,
  onChange,
}: {
  terms: Terms;
  /** The country default's rules; undefined when this profile is the default. */
  defaults?: Terms;
  editable: boolean;
  currency: string;
  onChange: (t: Terms) => void;
}) {
  const isDefault = !defaults;
  return (
    <div className="grid gap-6">
      {GROUPS.map((g) => (
        <section
          key={g}
          className="grid gap-4 border-b border-line pb-6 last:border-b-0 last:pb-0 lg:grid-cols-[190px_minmax(0,1fr)]"
        >
          <div>
            <h3 className="text-sm font-semibold">{GROUP_TITLE[g]}</h3>
            <p className="mt-2 text-xs leading-relaxed text-muted">{DESCRIPTION[g]}</p>
          </div>
          <div className="grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {TERM_FIELDS.filter((f) => f.group === g).map((f) =>
              !isDefault && f.countryOnly ? (
                <Field key={f.key} label={f.label} hint="From the country default">
                  <p className="tnum flex h-10 items-center rounded-[8px] bg-surface-2 px-3 text-sm text-muted">
                    {showTerm(f, defaults?.[f.key], currency)}
                  </p>
                </Field>
              ) : (
                <TermInput
                  key={f.key}
                  field={f}
                  value={terms[f.key]}
                  inherited={isDefault ? undefined : defaults?.[f.key]}
                  editable={editable}
                  currency={currency}
                  onChange={(v) => onChange({ ...terms, [f.key]: v })}
                />
              ),
            )}
          </div>
        </section>
      ))}
    </div>
  );
}

type Value = Terms[keyof Terms];

/** One rule: its input, plus "Use default" on a region's profile (null = inherit). */
function TermInput({
  field: f,
  value,
  inherited,
  editable,
  currency,
  onChange,
}: {
  field: TermField;
  value: Value;
  inherited?: Value;
  editable: boolean;
  currency: string;
  onChange: (v: Value) => void;
}) {
  const canInherit = inherited !== undefined;
  const inheriting = canInherit && (value === null || value === undefined);
  return (
    <Field
      label={`${f.label}${f.kind === "money" && currency ? ` (${currency})` : (UNIT[f.kind] ?? "")}`}
      hint={f.hint}
    >
      {inheriting ? (
        <p className="tnum flex h-10 items-center rounded-[8px] bg-surface-2 px-3 text-sm text-muted">
          {showTerm(f, inherited, currency)}
        </p>
      ) : (
        <ValueInput field={f} value={value} disabled={!editable} onChange={onChange} />
      )}
      {canInherit && editable ? (
        <label className="flex items-center gap-1.5 text-xs text-muted">
          <input
            type="checkbox"
            checked={inheriting}
            onChange={(e) => onChange(e.target.checked ? null : (inherited ?? null))}
            className="accent-[var(--brand)]"
          />
          {`Use default (${showTerm(f, inherited, currency)})`}
        </label>
      ) : null}
    </Field>
  );
}

/** The editor for one value; percentages and minutes are converted, text stays local until blur. */
function ValueInput({
  field: f,
  value,
  disabled,
  onChange,
}: {
  field: TermField;
  value: Value;
  disabled: boolean;
  onChange: (v: Value) => void;
}) {
  const shown = toText(f, value);
  const [text, setText] = useState(shown);
  const [editing, setEditing] = useState(false);
  if (f.kind === "choice" || f.kind === "bool") {
    const v = f.kind === "bool" ? (value ? "yes" : "no") : String(value ?? "");
    const opts =
      f.kind === "bool"
        ? [
            { value: "no", label: "No" },
            { value: "yes", label: "Yes" },
          ]
        : (f.options ?? []);
    return (
      <Select
        value={v}
        disabled={disabled}
        onChange={(e) => onChange(f.kind === "bool" ? e.target.value === "yes" : e.target.value)}
      >
        {opts.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </Select>
    );
  }
  return (
    <Input
      inputMode="decimal"
      value={editing ? text : shown}
      disabled={disabled}
      onFocus={() => {
        setText(shown);
        setEditing(true);
      }}
      onChange={(e) => setText(e.target.value.replace(/[^\d.]/g, ""))}
      onBlur={() => {
        setEditing(false);
        onChange(fromText(f, text));
      }}
    />
  );
}

function toText(f: TermField, v: Value): string {
  if (v === null || v === undefined) return "";
  if (f.kind === "pct") return fractionToPct(v as string);
  if (f.kind === "minutes") return String(Math.round(Number(v) / 60));
  return String(v);
}

function fromText(f: TermField, t: string): Value {
  const n = Number(t || 0);
  if (f.kind === "pct") return pctToFraction(t);
  if (f.kind === "minutes") return Math.round(n * 60);
  if (f.kind === "days") return Math.round(n);
  return String(n);
}
