"use client";
/** Stat — a KPI tile: label, a figure that counts up once, an optional trend against an earlier period, and a muted footnote. */
import { animate, motion, useReducedMotion } from "motion/react";
import { useEffect, useRef } from "react";
import { Delta } from "./Delta";
import { enter } from "./motion";

const COUNT_S = 0.5;

interface Props {
  label: string;
  value: number;
  format?: (n: number) => string;
  note?: string;
  icon?: React.ReactNode;
  index?: number;
  /** The same figure for the comparison period; shows a trend line when set. */
  previous?: number;
}

export function Stat({
  label,
  value,
  format = (n) => Math.round(n).toLocaleString("en-US"),
  note,
  icon,
  index = 0,
  previous,
}: Props) {
  const ref = useRef<HTMLSpanElement>(null);
  const reduce = useReducedMotion();
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (reduce) {
      el.textContent = format(value);
      return;
    }
    const c = animate(0, value, {
      duration: COUNT_S,
      ease: [0.32, 0.72, 0, 1],
      onUpdate: (v) => {
        el.textContent = format(v);
      },
      onComplete: () => {
        el.textContent = format(value); // land on the exact figure
      },
    });
    return () => c.stop();
  }, [value, format, reduce]);
  return (
    <motion.div {...enter(index)} className="min-w-0 rounded-[10px] bg-surface p-5">
      <div className="flex items-center justify-between text-sm text-muted">
        <span>{label}</span>
        {icon ? <span aria-hidden="true">{icon}</span> : null}
      </div>
      <span ref={ref} className="tnum mt-2 block text-[30px] font-semibold tracking-tight">
        {format(value)}
      </span>
      {previous !== undefined ? (
        <div className="mt-1">
          <Delta now={value} before={previous} />
        </div>
      ) : null}
      {note ? <p className="mt-1 text-xs text-muted">{note}</p> : null}
    </motion.div>
  );
}
