/**
 * The consent flag: must the signed-in person accept the current Privacy Notice
 * before using the app? Three sources write it:
 *  - sign-in and refresh (`TokenPair.consent_required`, see authService.ts);
 *  - `GET /me` (`Me.consent.required`, see useConsentSync.ts) on start and foreground;
 *  - a 403 CONSENT_REQUIRED from any call (booking, going online, onboarding
 *    submit), noticed by the session client's fetch wrapper (client.ts).
 * Each app's auth gate reads it and shows its consent screen while it is set.
 */
import { create } from "zustand";

/** Server code: the call needs an accepted Privacy Notice first. */
export const CONSENT_REQUIRED = "CONSENT_REQUIRED";
/** Server code: the version being accepted is no longer the current one. */
export const CONSENT_VERSION_STALE = "CONSENT_VERSION_STALE";

const FORBIDDEN = 403;

interface ConsentState {
  /** True while the person must accept the current notice before using the app. */
  required: boolean;
  setRequired: (required: boolean) => void;
}

/** In-memory consent flag. Not persisted: `GET /me` recomputes it on every start. */
export const useConsentStore = create<ConsentState>((set) => ({
  required: false,
  setRequired: (required) => set({ required }),
}));

/** React selector: must the signed-in person accept the notice now? */
export function useConsentRequired(): boolean {
  return useConsentStore((s) => s.required);
}

/**
 * Sets the flag when a response is a 403 CONSENT_REQUIRED. Reads a clone so the
 * caller still gets the untouched body.
 */
export async function noteConsentRequired(response: Response): Promise<void> {
  if (response.status !== FORBIDDEN) return;
  try {
    const body = (await response.clone().json()) as { error?: { code?: string } };
    if (body.error?.code === CONSENT_REQUIRED) useConsentStore.getState().setRequired(true);
  } catch {
    // Not JSON: not ours to interpret.
  }
}
