/** Palette roles exactly as served by the backend (see api/openapi.yaml → Palette). */
export interface Palette {
  primary: string;
  secondary: string;
  accent: string;
  background: string;
  surface: string;
  text: string;
  muted?: string;
  border?: string;
  success?: string;
  warning?: string;
  danger?: string;
  info?: string;
  on_primary?: string;
  on_accent?: string;
  highlight?: string;
}

/** The published theme payload for one brand. */
export interface Theme {
  version: number;
  preset?: string;
  palette: Palette;
  dark?: Palette;
  typography?: { heading_font?: string; body_font?: string };
  assets?: Record<string, string>;
}

/** Fallback used only before the first /app/config response arrives. Neutral, brand-free. */
export const bootstrapTheme: Theme = {
  version: 0,
  palette: {
    primary: "#1F2937",
    secondary: "#374151",
    accent: "#2563EB",
    background: "#FFFFFF",
    surface: "#F3F4F6",
    text: "#111318",
    muted: "#6B7280",
    border: "#E5E7EB",
    success: "#1B9C5A",
    warning: "#E0A100",
    danger: "#D64545",
    info: "#2563EB",
    on_primary: "#FFFFFF",
    on_accent: "#FFFFFF",
  },
};

/** Appends an alpha (0–1) to a 6-digit hex colour, e.g. withAlpha("#D9FDD3", 0.25). */
export function withAlpha(hex: string, alpha: number): string {
  const a = Math.round(Math.min(1, Math.max(0, alpha)) * 255)
    .toString(16)
    .padStart(2, "0");
  return /^#[0-9A-Fa-f]{6}$/.test(hex) ? `${hex}${a}` : hex;
}
