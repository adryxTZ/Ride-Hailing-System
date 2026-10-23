"use client";
/**
 * PreviewPane — how the draft looks before publishing: a rider home screen, a
 * driver offer card and a receipt header on a phone, in light or dark, using the
 * derived palette (colours come from the theme data, never from this file), plus
 * the contrast checks that decide whether it can be published.
 */
import { CircleAlert, CircleCheck, MapPin, Moon, Sun } from "lucide-react";
import { useState } from "react";
import type { Schemas } from "@/lib/api";
import { apiUrl } from "@/lib/config";

const CHECK_LABEL: Record<string, string> = {
  TEXT_ON_BACKGROUND: "Text on the page",
  TEXT_ON_SURFACE: "Text on cards",
  ON_PRIMARY: "Text on main-colour buttons",
  ON_ACCENT: "Text on accent buttons",
  PRIMARY_VS_ACCENT: "Main and accent look different",
};

export function PreviewPane({
  preview,
  name,
  logo,
  radius,
}: {
  preview?: { palette: Schemas["Palette"]; dark: Schemas["Palette"]; report: Schemas["ContrastReport"] };
  name: string;
  logo?: string;
  radius: number;
}) {
  const [dark, setDark] = useState(false);
  if (!preview) return <div className="h-[560px] animate-pulse rounded-[10px] bg-surface-2" />;
  const p = dark ? preview.dark : preview.palette;
  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-[15px] font-semibold">Preview</h2>
        <button
          type="button"
          onClick={() => setDark(!dark)}
          className="inline-flex items-center gap-1.5 rounded-[8px] bg-surface-2 px-3 py-1.5 text-xs font-medium"
          aria-pressed={dark}
        >
          {dark ? <Moon size={13} aria-hidden="true" /> : <Sun size={13} aria-hidden="true" />}
          {dark ? "Dark" : "Light"}
        </button>
      </div>
      <div className="rounded-[10px] bg-surface-2 p-4">
        <Phone p={p} name={name} logo={logo} radius={radius} />
      </div>
      <Checks report={preview.report} />
    </div>
  );
}

function Phone({
  p,
  name,
  logo,
  radius,
}: {
  p: Schemas["Palette"];
  name: string;
  logo?: string;
  radius: number;
}) {
  const card = { background: p.surface, color: p.text, borderRadius: radius + 6 };
  return (
    <div
      className="mx-auto w-full max-w-[300px] overflow-hidden rounded-[28px] p-3"
      style={{ background: p.background, color: p.text }}
      role="img"
      aria-label="Phone preview"
    >
      <div className="mb-3 flex items-center gap-2">
        {logo ? (
          <img src={apiUrl(logo)} alt="" className="h-7 w-auto" />
        ) : (
          <span
            className="grid size-7 place-items-center rounded-full text-xs font-bold"
            style={{ background: p.primary, color: p.on_primary }}
          >
            {name.slice(0, 1)}
          </span>
        )}
        <span className="text-sm font-semibold">{name}</span>
      </div>
      <div className="mb-2 p-3" style={card}>
        <p className="text-xs" style={{ color: p.muted }}>
          Where to?
        </p>
        <p className="mt-1 flex items-center gap-1.5 text-sm font-medium">
          <MapPin size={14} aria-hidden="true" style={{ color: p.accent }} /> Mlimani City
        </p>
      </div>
      <div className="mb-2 grid grid-cols-2 gap-2">
        {["Boda", "Standard"].map((k, i) => (
          <div
            key={k}
            className="p-2.5 text-xs font-medium"
            style={{
              ...card,
              background: i === 1 ? p.highlight : p.surface,
              outline: i === 1 ? `2px solid ${p.primary}` : undefined,
            }}
          >
            {k}
            <p style={{ color: p.muted }}>TZS 6,400</p>
          </div>
        ))}
      </div>
      <button
        type="button"
        tabIndex={-1}
        className="mb-3 w-full py-2.5 text-sm font-semibold"
        style={{ background: p.primary, color: p.on_primary, borderRadius: radius + 2 }}
      >
        Book ride
      </button>
      <div className="p-3" style={card}>
        <p className="text-xs font-semibold" style={{ color: p.accent }}>
          New ride offer
        </p>
        <p className="mt-1 text-sm">Sinza → Mwenge · 4.2 km</p>
        <div className="mt-2 flex gap-2">
          <span
            className="flex-1 py-1.5 text-center text-xs font-semibold"
            style={{ background: p.accent, color: p.on_accent, borderRadius: radius }}
          >
            Accept
          </span>
          <span
            className="flex-1 py-1.5 text-center text-xs"
            style={{ border: `1px solid ${p.border}`, borderRadius: radius }}
          >
            Decline
          </span>
        </div>
      </div>
    </div>
  );
}

function Checks({ report }: { report: Schemas["ContrastReport"] }) {
  return (
    <div>
      <p className={`mb-2 text-sm font-semibold ${report.pass ? "text-good" : "text-critical"}`}>
        {report.pass
          ? "Ready to publish: everything is readable"
          : "Fix the checks marked below before publishing"}
      </p>
      <ul className="grid gap-1 text-xs">
        {report.checks.map((c) => {
          const dark = c.key.startsWith("DARK_");
          const label = CHECK_LABEL[c.key.replace("DARK_", "")] ?? c.key;
          return (
            <li key={c.key} className="flex items-center gap-2">
              {c.pass ? (
                <CircleCheck size={14} className="text-good" aria-hidden="true" />
              ) : (
                <CircleAlert size={14} className="text-critical" aria-hidden="true" />
              )}
              <span className="flex-1">{`${label}${dark ? " (dark)" : ""}`}</span>
              <span className="tnum font-mono text-muted">{`${c.value.toFixed(1)} / ${c.min}`}</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
