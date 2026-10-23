/** Delta — change against an earlier period, as an arrow, a percentage and the comparison it refers to. */
import { ArrowDownRight, ArrowRight, ArrowUpRight } from "lucide-react";

/** Changes smaller than this read as "no change" rather than a tiny percentage. */
const FLAT_RATIO = 0.005;

export function Delta({
  now,
  before,
  label = "vs same time yesterday",
}: {
  now: number;
  before: number;
  label?: string;
}) {
  if (before === 0) {
    return (
      <span className="text-xs text-muted">
        {now === 0 ? `No change ${label}` : `None ${label.replace(/^vs /, "")}`}
      </span>
    );
  }
  const ratio = (now - before) / before;
  const flat = Math.abs(ratio) < FLAT_RATIO;
  const Icon = flat ? ArrowRight : ratio > 0 ? ArrowUpRight : ArrowDownRight;
  const tone = flat ? "text-muted" : ratio > 0 ? "text-good" : "text-critical";
  return (
    <span className="inline-flex items-center gap-1 text-xs">
      <span className={`tnum inline-flex items-center gap-0.5 font-semibold ${tone}`}>
        <Icon size={13} aria-hidden="true" />
        {flat ? "0%" : `${Math.round(Math.abs(ratio) * 100)}%`}
      </span>
      <span className="text-muted">{label}</span>
    </span>
  );
}
