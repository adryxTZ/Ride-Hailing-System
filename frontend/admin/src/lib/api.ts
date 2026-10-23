/**
 * api — the one typed client for the dashboard. Adds the bearer token, retries
 * once after a refresh on 401, and turns error envelopes into ApiError with the
 * server's stable code.
 */
import { type components, createApiClient } from "@platform-web/api-client";
import { API_BASE_URL, APP_VERSION } from "@/lib/config";
import { getAccess, refreshAccess } from "@/lib/session";

export type Schemas = components["schemas"];

/** fetch that refreshes the token once when the API answers 401. */
const authedFetch: typeof fetch = async (input, init) => {
  const req = input instanceof Request ? input : new Request(input, init);
  const retry = req.clone();
  const res = await fetch(req);
  if (res.status !== 401) return res;
  const token = await refreshAccess();
  if (!token) return res;
  retry.headers.set("Authorization", `Bearer ${token}`);
  return fetch(retry);
};

export const api = createApiClient({
  baseUrl: API_BASE_URL,
  getAccessToken: getAccess,
  locale: () => "en",
  appVersion: APP_VERSION,
  fetch: authedFetch,
});

/** An API failure with the server's code (screens branch on codes, never on text). */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly code?: string,
    readonly status?: number,
  ) {
    super(message);
  }
}

/** Unwraps an openapi-fetch result: the data, or an ApiError. */
export function unwrap<T>(r: { data?: T; error?: unknown; response: Response }): T {
  if (r.data !== undefined && !r.error) return r.data;
  const e = (r.error as { error?: { message?: string; code?: string } } | undefined)?.error;
  throw new ApiError(e?.message ?? `Request failed (${r.response.status})`, e?.code, r.response.status);
}
