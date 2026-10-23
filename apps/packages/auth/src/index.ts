/**
 * @platform/auth
 *
 *   session.ts      token persistence (secure store) + in-memory session state
 *   authService.ts  OTP request/verify, refresh, logout against the API
 *   client.ts       API client wired with the session: bearer header and one
 *                   transparent refresh-and-retry on 401
 *   useSession.ts   React hook over the session store
 */

export * from "./authService";
export * from "./client";
export * from "./session";
export * from "./useSession";
