"use client";
/** PeriodPicker — from/to dates (inclusive) with quick ranges; the period lives in the URL. */
import { Input } from "@/components/ui/Field";
import { day, type Period, thisMonth } from "./hooks";

/** Quick ranges, computed from today. */
function presets(now = new Date()): { label: string; period: Period }[] {
  const lastMonthEnd = new Date(now.getFullYear(), now.getMonth(), 0);
  return [
    { label: "This month", period: thisMonth(now) },
    {
      label: "Last month",
      period: {
        from: day(new Date(lastMonthEnd.getFullYear(), lastMonthEnd.getMonth(), 1)),
        to: day(lastMonthEnd),
      },
    },
    { label: "This year", period: { from: day(new Date(now.getFullYear(), 0, 1)), to: day(now) } },
  ];
}

export function PeriodPicker({ value, onChange }: { value: Period; onChange: (p: Period) => void }) {
  return (
    <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Period">
      {presets().map((p) => {
        const active = p.period.from === value.from && p.period.to === value.to;
        return (
          <button
            key={p.label}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(p.period)}
            className={`rounded-lg px-3 py-2 text-xs font-medium transition-colors ${active ? "bg-brand-soft text-brand-text" : "text-muted hover:bg-surface-2 hover:text-ink"}`}
          >
            {p.label}
          </button>
        );
      })}
      <label className="flex items-center gap-1.5 text-xs text-muted">
        From
        <span className="w-36">
          <Input
            type="date"
            aria-label="From"
            value={value.from}
            max={value.to}
            onChange={(e) => e.target.value && onChange({ ...value, from: e.target.value })}
          />
        </span>
      </label>
      <label className="flex items-center gap-1.5 text-xs text-muted">
        To
        <span className="w-36">
          <Input
            type="date"
            aria-label="To"
            value={value.to}
            min={value.from}
            onChange={(e) => e.target.value && onChange({ ...value, to: e.target.value })}
          />
        </span>
      </label>
    </div>
  );
}
