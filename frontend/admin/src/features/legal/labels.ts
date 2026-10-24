/** Legal labels — app and status words, language names, and the document page's URL. */
import type { Tone } from "@/components/ui/Chip";
import type { LegalApp, LegalKind } from "./hooks";

export const APP_LABEL: Record<LegalApp, string> = { RIDER: "Rider app", DRIVER: "Driver app" };

/** Who is asked to accept again when a document of this app is published. */
export const APP_PEOPLE: Record<LegalApp, string> = { RIDER: "riders", DRIVER: "drivers" };

export const STATUS_LABEL = { PUBLISHED: "Live", DRAFT: "Draft", ARCHIVED: "Earlier" } as const;
export const STATUS_TONE: Record<keyof typeof STATUS_LABEL, Tone> = {
  PUBLISHED: "ok",
  DRAFT: "live",
  ARCHIVED: "muted",
};

export const LANG_LABEL: Record<string, string> = { en: "English", sw: "Swahili" };

/** /settings/legal/rider/privacy-notice */
export function docHref(app: LegalApp, slug: string): string {
  return `/settings/legal/${app.toLowerCase()}/${slug}`;
}

/** The kind behind a URL slug ("privacy-notice" → PRIVACY_NOTICE). */
export function kindOf(slug: string): LegalKind {
  return slug.toUpperCase().replaceAll("-", "_") as LegalKind;
}
