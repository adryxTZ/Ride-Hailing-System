/** Chip — a small status label; tone colours always come with the word, never alone. */
export type Tone = "ok" | "live" | "warn" | "bad" | "muted" | "brand";

const TONE: Record<Tone, string> = {
  ok: "bg-[color-mix(in_srgb,var(--good)_14%,transparent)] text-good",
  live: "bg-brand-soft text-brand-text",
  warn: "bg-[color-mix(in_srgb,var(--warn)_16%,transparent)] text-warn",
  bad: "bg-[color-mix(in_srgb,var(--critical)_14%,transparent)] text-critical",
  muted: "bg-surface-2 text-muted",
  brand: "bg-brand text-brand-ink",
};

/** `wrap` lets a long label break onto two lines instead of overflowing its card. */
export function Chip({
  tone = "muted",
  wrap = false,
  children,
}: {
  tone?: Tone;
  wrap?: boolean;
  children: React.ReactNode;
}) {
  return (
    <span
      className={`inline-flex max-w-full items-center gap-1 ${wrap ? "rounded-[8px]" : "whitespace-nowrap rounded-full"} px-2.5 py-0.5 text-xs font-semibold ${TONE[tone]}`}
    >
      {children}
    </span>
  );
}
