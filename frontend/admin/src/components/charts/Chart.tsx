"use client";
import type { ApexOptions } from "apexcharts";
/**
 * Chart — ApexCharts with one shared base(): same type, grid, tooltip and legend
 * everywhere, validated series colours in fixed slots, and a table view behind a
 * toggle (the keyboard and screen-reader path). Loaded client-side only.
 */
import dynamic from "next/dynamic";
import { useState } from "react";

const ApexChart = dynamic(() => import("react-apexcharts"), { ssr: false });

const css = (v: string) =>
  typeof window === "undefined" ? "" : getComputedStyle(document.documentElement).getPropertyValue(v).trim();

/** The shared options every chart starts from. */
export function base(): ApexOptions {
  const ink = css("--muted");
  return {
    chart: {
      toolbar: { show: false },
      fontFamily: "var(--font-sans)",
      background: "transparent",
      animations: { enabled: true, speed: 650 },
    },
    colors: [css("--chart-1"), css("--chart-2"), css("--chart-3"), css("--chart-4")],
    grid: { borderColor: css("--line"), strokeDashArray: 3, padding: { left: 8, right: 8 } },
    dataLabels: { enabled: false },
    legend: {
      position: "top",
      horizontalAlign: "left",
      labels: { colors: ink },
      markers: { offsetX: -4 },
      itemMargin: { horizontal: 13 },
    },
    xaxis: {
      labels: { style: { colors: ink, fontFamily: "var(--font-mono)", fontSize: "11px" } },
      axisBorder: { show: false },
      axisTicks: { show: false },
    },
    yaxis: {
      labels: { style: { colors: ink, fontFamily: "var(--font-mono)", fontSize: "11px" } },
      min: 0,
      forceNiceScale: true,
    },
    tooltip: { theme: css("color-scheme") === "dark" ? "dark" : "light" },
    stroke: { curve: "smooth", width: 2 },
  };
}

type Axis = Record<string, unknown> & { labels?: Record<string, unknown> };

/** A page's options over base(): axes merge one level deep so a page can set a
 *  formatter or a max without losing the shared label styles. */
function merge(b: ApexOptions, type: Props["type"], o: ApexOptions = {}): ApexOptions {
  const bx = b.xaxis as Axis;
  const by = b.yaxis as Axis;
  const ox = (o.xaxis ?? {}) as Axis;
  const oy = (o.yaxis ?? {}) as Axis;
  return {
    ...b,
    ...(type === "bar" ? { stroke: { show: true, width: 2, colors: [css("--surface")] } } : {}),
    ...o,
    xaxis: { ...bx, ...ox, labels: { ...bx.labels, ...ox.labels } },
    yaxis: { ...by, ...oy, labels: { ...by.labels, ...oy.labels } },
  };
}

interface Props {
  type: "area" | "bar" | "line";
  series: ApexAxisChartSeries;
  options?: ApexOptions;
  height?: number;
  /** Rows for the table view: first column is the x label. */
  table: { head: string[]; rows: (string | number)[][] };
}

export function Chart({ type, series, options, height = 260, table }: Props) {
  const [asTable, setAsTable] = useState(false);
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
          <thead>
            <tr>
              {table.head.map((h) => (
                <th key={h} className="pb-2 text-left text-xs text-muted">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {table.rows.map((r) => (
              <tr key={String(r[0])} className="border-t border-line">
                {r.map((c, i) => (
                  <td key={`${r[0]}-${i}`} className="tnum py-1.5">
                    {c}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <ApexChart type={type} series={series} options={merge(base(), type, options)} height={height} />
      )}
    </div>
  );
}
