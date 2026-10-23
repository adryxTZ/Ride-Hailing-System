/** The theme being edited on the Branding page, and helpers to start one from a version or a preset. */
import type { Schemas } from "@/lib/api";

export type Draft = Schemas["ThemeDraftInput"] & { id?: string };

/** Roles shown as pickers (the rest derive from these). */
export const MAIN_ROLES = ["primary", "secondary", "accent", "background", "surface", "text"] as const;
export const ROLE_LABEL: Record<string, string> = {
  primary: "Main colour",
  secondary: "Second colour",
  accent: "Accent (buttons, highlights)",
  background: "Page background",
  surface: "Cards",
  text: "Text",
};

export const SLOT_LABEL: Record<string, { label: string; hint: string; release?: boolean }> = {
  logo_full: {
    label: "Full logo",
    hint: "Wide logo for headers and receipts. PNG with a transparent background.",
  },
  logo_mark: { label: "Logo mark", hint: "Square symbol, at least 512 × 512." },
  logo_on_dark: { label: "Logo on dark", hint: "Light version for dark screens." },
  app_icon: { label: "App icon", hint: "1024 × 1024 PNG, no transparency.", release: true },
  splash: { label: "Splash screen", hint: "Shown while the app opens, 1242 × 2436.", release: true },
  favicon: { label: "Browser icon", hint: "64 × 64 PNG for the website and dashboard." },
};

/** A draft copied from a stored version (edits never touch the stored one). */
export function fromVersion(v: Schemas["ThemeVersion"], keepId: boolean): Draft {
  return {
    id: keepId && v.status === "DRAFT" ? v.id : undefined,
    name: v.name || `Version ${v.version}`,
    source: v.source,
    roles: { light: { ...(v.roles.light ?? {}) }, dark: { ...(v.roles.dark ?? {}) } },
    typography: { ...v.typography },
    shape: { ...v.shape },
    assets: { ...v.assets },
  };
}

/** Applies a preset's colours, keeping fonts, radius and logos. */
export function withPreset(d: Draft, p: Schemas["ThemePreset"]): Draft {
  return {
    ...d,
    id: d.id,
    source: `PRESET:${p.key}`,
    name: p.name,
    roles: { light: { ...p.roles }, dark: {} },
  };
}
