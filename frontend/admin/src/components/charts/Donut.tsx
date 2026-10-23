"use client";
/**
 * Donut — a pie with a hole, sharing Chart's base(): the validated colours in
 * fixed slots (at most four; smaller slices fold into "Other"), the total in the
 * middle, a legend, and the same table view behind a toggle.
 */
import dynamic from "next/dynamic";
import { useState } from "react";
import { base } from "./Chart";

const ApexChart = dynamic(() => import("react-apexcharts"), { ssr: false });

/** The palette has four validated slots; a fifth slice would reuse a colour. */
export const MAX_SLICES = 4;

export interface Slice {
  label: string;
  value: number;
}

/** Keeps the largest slices and folds the rest into one "Other" slice. */
export function foldSlices(slices: Slice[], max = MAX_SLICES): Slice[] {
  const sorted = [...slices].sort((a, b) => b.value - a.value);
  if (sorted.length <= max) return sorted;
  const rest = sorted.slice(max - 1).reduce((n, s) => n + s.value, 0);
  return [...sorted.slice(0, max - 1), { label: "Other", value: rest }];
}

export function Donut({
  slices,
  total,
  height = 240,
  format = (n) => n.toLocaleString("en-US"),
}: {
  slices: Slice[];
  /** Label for the centre total, e.g. "trips". */
  total: string;
  height?: number;
  format?: (n: number) => string;
}) {
  const [asTable, setAsTable] = useState(false);
  const shown = foldSlices(slices).filter((s) => s.value > 0);
  const sum = shown.reduce((n, s) => n + s.value, 0);
  if (sum === 0)
    return <p className="grid h-40 place-items-center text-sm text-muted">Nothing to show yet</p>;
  const b = base();
  return (
    <div>
      <div className="mb-1 flex justify-end">
        <button
          type="button"
          onClick={() => setAsTable(!asTable)}
          className="rounded-full px-3 py-1 text-xs text-muted hover:bg-surface-2"
        >
          {asTable ? "Show chart" : "Show table"}
        </button>
      </div>
      {asTable ? (
        <table className="w-full text-sm">
          <tbody>
            {shown.map((s) => (
              <tr key={s.label} className="border-t border-line">
                <td className="py-1.5">{s.label}</td>
                <td className="tnum py-1.5 text-right">{format(s.value)}</td>
                <td className="tnum py-1.5 text-right text-muted">{`${Math.round((s.value / sum) * 100)}%`}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <ApexChart
          type="donut"
          height={height}
          series={shown.map((s) => s.value)}
          options={{
            chart: b.chart,
            colors: b.colors,
            legend: { ...b.legend, position: "bottom", horizontalAlign: "center" },
            tooltip: {
              ...b.tooltip,
              y: { formatter: (v: number) => `${format(v)} (${Math.round((v / sum) * 100)}%)` },
            },
            labels: shown.map((s) => s.label),
            dataLabels: { enabled: false },
            stroke: {
              width: 2,
              colors: [getComputedStyle(document.documentElement).getPropertyValue("--surface").trim()],
            },
            plotOptions: {
              pie: {
                donut: {
                  size: "68%",
                  labels: {
                    show: true,
                    value: { fontFamily: "var(--font-sans)", fontWeight: 600 },
                    total: { show: true, label: total, formatter: () => format(sum) },
                  },
                },
              },
            },
          }}
        />
      )}
    </div>
  );
}
