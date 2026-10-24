"use client";
/**
 * ChargesEditor — a profile's extra charges as a draft list (published with the
 * rules as a new version). Each row expands into its form: name in EN and SW,
 * kind, amount rule, who receives it, when it applies, and optional limits by
 * vehicle type, fixed fares, weekdays and time of day.
 */
import { ChevronDown, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Select } from "@/components/ui/Field";
import { Empty } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { useClassKeys, useClassName } from "@/lib/catalog";
import { useScope } from "@/lib/scope";
import { useChargeDefaults } from "./hooks";
import {
  AMOUNT_TYPE_LABEL,
  CATEGORY_LABEL,
  type Charge,
  DAY_LABEL,
  fractionToPct,
  NEEDS_GEOFENCES,
  newCharge,
  pctToFraction,
  RECIPIENT_LABEL,
  WHEN_LABEL,
} from "./model";

export function ChargesEditor({
  charges,
  payees,
  editable,
  currency,
  onChange,
}: {
  charges: Charge[];
  payees: Schemas["ChargePayee"][];
  editable: boolean;
  currency: string;
  onChange: (c: Charge[]) => void;
}) {
  const [open, setOpen] = useState<number>();
  const set = (i: number, c: Charge) => onChange(charges.map((x, j) => (j === i ? c : x)));
  return (
    <div className="grid gap-2">
      {charges.length === 0 ? (
        <Empty
          title="No extra charges"
          body="Add a toll, parking, levy or airport fee that trips priced by this profile pay."
        />
      ) : (
        charges.map((c, i) => (
          <div key={i} className="rounded-[8px] bg-surface-2">
            <button
              type="button"
              aria-expanded={open === i}
              onClick={() => setOpen(open === i ? undefined : i)}
              className="flex w-full flex-wrap items-center gap-2 px-3 py-2.5 text-left text-sm"
            >
              <span className="font-medium">{c.label.en || "New charge"}</span>
              <Chip>{CATEGORY_LABEL[c.category]}</Chip>
              <span className="tnum text-muted">{amountText(c, currency)}</span>
              <span className="text-muted">{`· ${RECIPIENT_LABEL[c.recipient]} · ${WHEN_LABEL[c.applies_when]}`}</span>
              <ChevronDown
                size={15}
                aria-hidden="true"
                className={`ml-auto transition-transform duration-200 ${open === i ? "rotate-180" : ""}`}
              />
            </button>
            {open === i ? (
              <ChargeForm
                charge={c}
                payees={payees}
                editable={editable}
                onChange={(x) => set(i, x)}
                onRemove={() => {
                  onChange(charges.filter((_, j) => j !== i));
                  setOpen(undefined);
                }}
              />
            ) : null}
          </div>
        ))
      )}
      {editable ? (
        <Button
          variant="soft"
          size="sm"
          className="justify-self-start"
          icon={<Plus size={14} aria-hidden="true" />}
          onClick={() => {
            onChange([...charges, newCharge()]);
            setOpen(charges.length);
          }}
        >
          Add a charge
        </Button>
      ) : null}
    </div>
  );
}

function amountText(c: Charge, currency: string) {
  if (c.amount_type === "PERCENT_OF_FARE")
    return `${Math.round(Number(c.amount) * 10000) / 100}% of the fare`;
  if (c.amount_type === "PER_KM") return `${currency} ${c.amount} per km`;
  return `${currency} ${Number(c.amount).toLocaleString("en-US")}`;
}

/** The fields of one charge. */
function ChargeForm({
  charge: c,
  payees,
  editable,
  onChange,
  onRemove,
}: {
  charge: Charge;
  payees: Schemas["ChargePayee"][];
  editable: boolean;
  onChange: (c: Charge) => void;
  onRemove: () => void;
}) {
  const className = useClassName();
  const classes = useClassKeys();
  const { scope } = useScope();
  const { data: defaults } = useChargeDefaults(scope.country);
  // Picking a kind takes its default recipient (tolls and parking go to the driver unless changed).
  const pickCategory = (category: Charge["category"]) => {
    const d = defaults?.find((x) => x.category === category);
    set(
      d
        ? { category, recipient: d.recipient, payee_id: d.payee_id ?? undefined, taxable: false }
        : { category },
    );
  };
  const set = (patch: Partial<Charge>) => onChange({ ...c, ...patch });
  const toggle = <T,>(list: T[] | undefined, v: T) =>
    (list ?? []).includes(v) ? (list ?? []).filter((x) => x !== v) : [...(list ?? []), v];
  return (
    <fieldset disabled={!editable} className="grid gap-3 border-t border-line px-3 py-3 md:grid-cols-3">
      <Field label="Name (English)">
        <Input
          value={c.label.en}
          maxLength={40}
          onChange={(e) => set({ label: { ...c.label, en: e.target.value } })}
        />
      </Field>
      <Field label="Name (Swahili)">
        <Input
          value={c.label.sw}
          maxLength={40}
          onChange={(e) => set({ label: { ...c.label, sw: e.target.value } })}
        />
      </Field>
      <Field label="Code" hint="Capitals and _ ; once per trip">
        <Input
          value={c.code}
          onChange={(e) => set({ code: e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, "") })}
        />
      </Field>
      <Field label="Kind">
        <Select value={c.category} onChange={(e) => pickCategory(e.target.value as Charge["category"])}>
          {Object.entries(CATEGORY_LABEL).map(([k, v]) => (
            <option key={k} value={k}>
              {v}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Amount is">
        <Select
          value={c.amount_type}
          onChange={(e) => set({ amount_type: e.target.value as Charge["amount_type"] })}
        >
          {Object.entries(AMOUNT_TYPE_LABEL).map(([k, v]) => (
            <option key={k} value={k}>
              {v}
            </option>
          ))}
        </Select>
      </Field>
      <Field label={c.amount_type === "PERCENT_OF_FARE" ? "Amount (%)" : "Amount"}>
        <AmountInput
          percent={c.amount_type === "PERCENT_OF_FARE"}
          value={c.amount}
          onChange={(amount) => set({ amount })}
        />
      </Field>
      <Field label="Smallest amount" hint="Optional">
        <Input
          inputMode="decimal"
          value={c.min_amount ?? ""}
          onChange={(e) => set({ min_amount: e.target.value.replace(/[^\d.]/g, "") || undefined })}
        />
      </Field>
      <Field label="Largest amount" hint="Optional">
        <Input
          inputMode="decimal"
          value={c.max_amount ?? ""}
          onChange={(e) => set({ max_amount: e.target.value.replace(/[^\d.]/g, "") || undefined })}
        />
      </Field>
      <Field label="Who receives it">
        <Select
          value={c.recipient}
          onChange={(e) =>
            set({ recipient: e.target.value as Charge["recipient"], payee_id: undefined, taxable: false })
          }
        >
          {Object.entries(RECIPIENT_LABEL).map(([k, v]) => (
            <option key={k} value={k}>
              {v}
            </option>
          ))}
        </Select>
      </Field>
      {c.recipient === "THIRD_PARTY" ? (
        <Field label="Payee" hint={payees.length ? undefined : "Add a payee on the Payees tab first"}>
          <Select value={c.payee_id ?? ""} onChange={(e) => set({ payee_id: e.target.value || undefined })}>
            <option value="">Choose…</option>
            {payees
              .filter((p) => p.status === "ACTIVE")
              .map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
          </Select>
        </Field>
      ) : null}
      {c.recipient === "PLATFORM" ? (
        <Field label="VAT">
          <Select
            value={c.taxable ? "yes" : "no"}
            onChange={(e) => set({ taxable: e.target.value === "yes" })}
          >
            <option value="no">Not taxed</option>
            <option value="yes">VAT included</option>
          </Select>
        </Field>
      ) : null}
      <Field label="When it applies">
        <Select
          value={c.applies_when}
          onChange={(e) => set({ applies_when: e.target.value as Charge["applies_when"] })}
        >
          {Object.entries(WHEN_LABEL).map(([k, v]) => (
            <option key={k} value={k} disabled={NEEDS_GEOFENCES.has(k)}>
              {NEEDS_GEOFENCES.has(k) ? `${v} (needs geofences)` : v}
            </option>
          ))}
        </Select>
      </Field>
      <div className="grid gap-1.5 text-sm md:col-span-3">
        <span className="font-medium">Only for these vehicle types (none = all)</span>
        <div className="flex flex-wrap gap-1.5">
          {classes.map((k) => (
            <PillToggle
              key={k}
              on={(c.vehicle_classes ?? []).includes(k)}
              onClick={() => set({ vehicle_classes: toggle(c.vehicle_classes, k) })}
            >
              {className(k)}
            </PillToggle>
          ))}
          <PillToggle
            on={c.on_fixed_routes ?? true}
            onClick={() => set({ on_fixed_routes: !(c.on_fixed_routes ?? true) })}
          >
            Also on fixed fares
          </PillToggle>
        </div>
      </div>
      <div className="grid gap-1.5 text-sm md:col-span-2">
        <span className="font-medium">Days (none = every day)</span>
        <div className="flex flex-wrap gap-1.5">
          {DAY_LABEL.map((d, i) => (
            <PillToggle
              key={d}
              on={(c.days ?? []).includes(i + 1)}
              onClick={() => set({ days: toggle(c.days, i + 1) })}
            >
              {d}
            </PillToggle>
          ))}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Field label="From" hint="Local time">
          <Input
            type="time"
            value={c.from_time ?? ""}
            onChange={(e) =>
              set({
                from_time: e.target.value || undefined,
                to_time: c.to_time ?? (e.target.value ? "05:00" : undefined),
              })
            }
          />
        </Field>
        <Field label="Until">
          <Input
            type="time"
            value={c.to_time ?? ""}
            onChange={(e) => set({ to_time: e.target.value || undefined })}
          />
        </Field>
      </div>
      {editable ? (
        <div className="md:col-span-3">
          <Button size="sm" variant="ghost" icon={<Trash2 size={14} aria-hidden="true" />} onClick={onRemove}>
            Remove this charge
          </Button>
        </div>
      ) : null}
    </fieldset>
  );
}

function PillToggle({
  on,
  onClick,
  children,
}: {
  on: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={`rounded-[8px] px-2.5 py-1 text-xs font-medium transition-colors duration-150 ${on ? "bg-nav text-nav-ink" : "bg-surface text-muted hover:text-ink"}`}
    >
      {children}
    </button>
  );
}

/** An amount or a percentage; typed text stays local (so "7." keeps its dot) and is stored on blur. */
function AmountInput({
  percent,
  value,
  onChange,
}: {
  percent: boolean;
  value: string;
  onChange: (v: string) => void;
}) {
  const shown = percent ? fractionToPct(value) : value;
  const [text, setText] = useState(shown);
  const [editing, setEditing] = useState(false);
  return (
    <Input
      inputMode="decimal"
      value={editing ? text : shown}
      onFocus={() => {
        setText(shown);
        setEditing(true);
      }}
      onChange={(e) => setText(e.target.value.replace(/[^\d.]/g, ""))}
      onBlur={() => {
        setEditing(false);
        onChange(percent ? pctToFraction(text) : text || "0");
      }}
    />
  );
}
