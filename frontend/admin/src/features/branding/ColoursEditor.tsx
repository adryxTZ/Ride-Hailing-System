"use client";
/** ColoursEditor — pick a preset, import a Color Hunt palette, or set each colour; dark mode overrides are optional. */
import { Check, Download } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Field";
import { ErrorNote } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { type Draft, MAIN_ROLES, ROLE_LABEL, withPreset } from "./draft";
import { useImportColorHunt } from "./hooks";

const HEX = /^#[0-9A-Fa-f]{6}$/;

export function ColoursEditor({
  draft,
  presets,
  derived,
  onChange,
}: {
  draft: Draft;
  presets: Schemas["ThemePreset"][];
  /** The derived palette, shown as the starting value for roles left empty. */
  derived?: Schemas["Palette"];
  onChange: (d: Draft) => void;
}) {
  const [link, setLink] = useState("");
  const [showDark, setShowDark] = useState(Object.keys(draft.roles.dark ?? {}).length > 0);
  const importer = useImportColorHunt();
  const setRole = (mode: "light" | "dark", role: string, value: string) =>
    onChange({
      ...draft,
      source: draft.source.startsWith("PRESET:") ? "CUSTOM" : draft.source,
      roles: { ...draft.roles, [mode]: { ...(draft.roles[mode] ?? {}), [role]: value } },
    });
  return (
    <div className="grid gap-6">
      <section>
        <h3 className="mb-2 text-sm font-semibold">Ready-made palettes</h3>
        <ul className="grid grid-cols-2 gap-2 md:grid-cols-3">
          {presets.map((p) => {
            const active = draft.source === `PRESET:${p.key}`;
            return (
              <li key={p.key}>
                <button
                  type="button"
                  aria-pressed={active}
                  onClick={() => onChange(withPreset(draft, p))}
                  className={`w-full rounded-[8px] p-2 text-left transition-colors duration-150 ${active ? "bg-brand-soft ring-2 ring-brand" : "bg-surface-2 hover:bg-line"}`}
                >
                  <span className="flex h-8 overflow-hidden rounded-[6px]">
                    {p.colours.map((c) => (
                      <span key={c} className="flex-1" style={{ background: c }} />
                    ))}
                  </span>
                  <span className="mt-1.5 flex items-center gap-1 text-xs font-medium">
                    {active ? <Check size={12} aria-hidden="true" /> : null}
                    {p.name}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </section>
      <section>
        <h3 className="mb-2 text-sm font-semibold">Import from Color Hunt</h3>
        <ErrorNote error={importer.error} />
        <div className="flex gap-2">
          <Input
            aria-label="Color Hunt link"
            placeholder="https://colorhunt.co/palette/0b244719376d576cbca5d7e8"
            value={link}
            onChange={(e) => setLink(e.target.value)}
          />
          <Button
            variant="soft"
            icon={<Download size={15} aria-hidden="true" />}
            disabled={!link.trim()}
            loading={importer.isPending}
            onClick={() =>
              importer.mutate(link.trim(), {
                onSuccess: (r) =>
                  onChange({
                    ...draft,
                    source: `IMPORTED:${link.trim()}`,
                    name: "Imported palette",
                    roles: { light: r.roles, dark: {} },
                  }),
              })
            }
          >
            Import
          </Button>
        </div>
        <p className="mt-1 text-xs text-muted">
          The four colours are read from the link and given roles automatically. Adjust them below.
        </p>
      </section>
      <section>
        <h3 className="mb-2 text-sm font-semibold">Colours</h3>
        <div className="grid gap-2 md:grid-cols-2">
          {MAIN_ROLES.map((role) => (
            <RolePicker
              key={role}
              label={ROLE_LABEL[role]}
              value={draft.roles.light?.[role] ?? ""}
              placeholder={derived?.[role as keyof Schemas["Palette"]] as string | undefined}
              onChange={(v) => setRole("light", role, v)}
            />
          ))}
        </div>
      </section>
      <section>
        <button
          type="button"
          className="text-sm font-semibold text-brand-text"
          onClick={() => setShowDark(!showDark)}
          aria-expanded={showDark}
        >
          {showDark ? "Hide dark mode colours" : "Adjust dark mode colours"}
        </button>
        {showDark ? (
          <div className="mt-2 grid gap-2 md:grid-cols-2">
            {MAIN_ROLES.map((role) => (
              <RolePicker
                key={role}
                label={`${ROLE_LABEL[role]} (dark)`}
                value={draft.roles.dark?.[role] ?? ""}
                placeholder="Automatic"
                onChange={(v) => setRole("dark", role, v)}
              />
            ))}
            <p className="text-xs text-muted md:col-span-2">
              Leave a colour empty to have it worked out from the light colours.
            </p>
          </div>
        ) : null}
      </section>
    </div>
  );
}

/** A colour swatch picker plus its hex text; an invalid hex is kept in the box but not applied. */
function RolePicker({
  label,
  value,
  placeholder,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (v: string) => void;
}) {
  const [text, setText] = useState(value);
  const shown = HEX.test(value) ? value : placeholder && HEX.test(placeholder) ? placeholder : "#FFFFFF";
  return (
    <label className="flex items-center gap-2 rounded-[8px] bg-surface-2 px-2 py-1.5 text-sm">
      <input
        type="color"
        aria-label={label}
        value={shown}
        onChange={(e) => {
          const v = e.target.value.toUpperCase();
          setText(v);
          onChange(v);
        }}
        className="size-8 shrink-0 cursor-pointer rounded-[6px] border-0 bg-transparent p-0"
      />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <input
        aria-label={`${label} hex`}
        value={text}
        placeholder={placeholder}
        onChange={(e) => {
          const v = e.target.value.trim().toUpperCase();
          setText(v);
          if (v === "" || HEX.test(v)) onChange(v);
        }}
        className="tnum w-24 rounded-[6px] bg-surface px-2 py-1 font-mono text-xs outline-none focus:ring-2 focus:ring-brand"
      />
    </label>
  );
}
