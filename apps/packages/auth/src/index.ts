/**
 * @platform/auth
 *
 *   session.ts      token persistence (secure store) + in-memory session state
 *   authService.ts  OTP request/verify, refresh, logout against the API
 *   client.ts       API client wired with the session: bearer header and one
 *                   transparent refresh-and-retry on 401
 *   useSession.ts   React hook over the session store
 *   install.ts      per-install id sent as X-Install-Id (sign-in, consent)
 *   consentStore.ts "must accept the Privacy Notice now" flag and its sources
 *   consent.ts      read and accept the current Privacy Notice
 *   useConsentSync.ts  keeps the flag in step with GET /me on start and foreground
 */

export * from "./authService";
export * from "./client";
export * from "./consent";
export * from "./consentStore";
export { installId } from "./install";
export * from "./session";
export * from "./useConsentSync";
export * from "./useSession";
