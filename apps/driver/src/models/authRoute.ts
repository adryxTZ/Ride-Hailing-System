/**
 * authRoute — where the auth gate sends the person, from three facts: signed in,
 * must accept the Privacy Notice, and the first route segment they are on.
 * Pure, so the rules are testable without a router.
 */

/** First route segment of the auth group and of the consent screen. */
export const AUTH_GROUP = "(auth)";
export const CONSENT_SEGMENT = "consent";

export type AuthMove =
  /** Signed out: start sign-in. */
  | { kind: "sign-in" }
  /** Show the consent screen; `stack` keeps the current screen to return to after accepting. */
  | { kind: "consent"; stack: boolean }
  /** Signed in and nothing owed: leave the auth or consent screen. */
  | { kind: "home"; fromConsent: boolean }
  | { kind: "stay" };

export function authRoute(signedIn: boolean, consentRequired: boolean, segment?: string): AuthMove {
  const inAuth = segment === AUTH_GROUP;
  const onConsent = segment === CONSENT_SEGMENT;
  if (!signedIn) return inAuth ? { kind: "stay" } : { kind: "sign-in" };
  if (consentRequired) return onConsent ? { kind: "stay" } : { kind: "consent", stack: !inAuth };
  if (inAuth || onConsent) return { kind: "home", fromConsent: onConsent };
  return { kind: "stay" };
}
