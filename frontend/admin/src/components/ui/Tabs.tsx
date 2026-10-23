"use client";
/** Tabs — pill filter tabs with a gliding active pill (the page's main filter). */
import { motion } from "motion/react";
import { EASE } from "./motion";

export function Tabs<T extends string>({
  value,
  options,
  onChange,
  id,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
  id: string;
}) {
  return (
    <div
      role="tablist"
      className="inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-lg bg-surface p-1"
    >
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(o.value)}
            className={`relative shrink-0 whitespace-nowrap rounded-lg px-3 py-2 text-xs font-medium transition-colors duration-200 ${active ? "text-brand-text" : "text-muted hover:text-ink"}`}
          >
            {active ? (
              <motion.span
                layoutId={`tab-${id}`}
                className="absolute inset-0 rounded-lg bg-brand-soft"
                transition={{ duration: 0.35, ease: EASE }}
              />
            ) : null}
            <span className="relative">{o.label}</span>
          </button>
        );
      })}
    </div>
  );
}
