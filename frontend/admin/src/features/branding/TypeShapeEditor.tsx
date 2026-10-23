"use client";
/** TypeShapeEditor — fonts from the bundled list, and the three corner radii. */
import { Field, Select } from "@/components/ui/Field";
import type { Draft } from "./draft";

const FONT_KEYS = [
  { key: "heading_font", label: "Headings" },
  { key: "body_font", label: "Body text" },
] as const;
const RADIUS_LABEL: Record<string, string> = {
  radius_sm: "Small (chips, inputs)",
  radius_md: "Medium (buttons)",
  radius_lg: "Large (cards)",
};

export function TypeShapeEditor({
  draft,
  fonts,
  shapeMax,
  onChange,
}: {
  draft: Draft;
  fonts: string[];
  shapeMax: Record<string, number>;
  onChange: (d: Draft) => void;
}) {
  return (
    <div className="grid gap-6">
      <section className="grid gap-3 md:grid-cols-2">
        {FONT_KEYS.map((f) => (
          <Field key={f.key} label={f.label} hint="Fonts are built into the apps; only these can be chosen.">
            <Select
              value={draft.typography?.[f.key] ?? ""}
              onChange={(e) =>
                onChange({ ...draft, typography: { ...(draft.typography ?? {}), [f.key]: e.target.value } })
              }
            >
              <option value="">App default</option>
              {fonts.map((name) => (
                <option key={name} value={name}>
                  {name}
                </option>
              ))}
            </Select>
          </Field>
        ))}
      </section>
      <section className="grid gap-4">
        {Object.entries(shapeMax).map(([key, max]) => {
          const v = draft.shape?.[key] ?? 0;
          return (
            <label key={key} className="grid gap-1.5 text-sm">
              <span className="flex justify-between font-medium">
                {RADIUS_LABEL[key] ?? key}
                <span className="tnum text-muted">{`${v} px`}</span>
              </span>
              <input
                type="range"
                min={0}
                max={max}
                value={v}
                onChange={(e) =>
                  onChange({ ...draft, shape: { ...(draft.shape ?? {}), [key]: Number(e.target.value) } })
                }
                className="accent-[var(--brand)]"
              />
            </label>
          );
        })}
      </section>
    </div>
  );
}
