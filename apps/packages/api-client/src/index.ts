/**
 * @platform/api-client
 *
 * One typed HTTP client for the rider app, the driver app and the admin.
 * Types come from `src/generated/schema.d.ts`, produced by `pnpm gen` from
 * `backend/api/openapi.yaml`. This file only wires the fetch client and the
 * TanStack Query helpers; it contains no endpoint knowledge of its own.
 */
import createClient, { type Middleware } from "openapi-fetch";
import createQueryClient from "openapi-react-query";
import type { paths } from "./generated/schema";

export type { components, paths } from "./generated/schema";

export interface ClientOptions {
  /** Base URL including the /v1 prefix, e.g. https://api.example.com/v1 */
  baseUrl: string;
  /** Returns the current access token, or null when signed out. */
  getAccessToken: () => Promise<string | null> | string | null;
  /** BCP-47 language for Accept-Language (drives localised error messages). */
  locale: () => string;
  /** App semantic version for X-App-Version (enables 426 APP_UPDATE_REQUIRED). */
  appVersion: string;
  /** Custom fetch (e.g. a wrapper that retries after a token refresh). Defaults to global fetch. */
  fetch?: typeof globalThis.fetch;
}

/** Builds the fetch client with auth, locale and app-version headers attached. */
export function createApiClient(opts: ClientOptions) {
  const client = createClient<paths>({ baseUrl: opts.baseUrl, fetch: opts.fetch });
  const headers: Middleware = {
    async onRequest({ request }) {
      const token = await opts.getAccessToken();
      if (token) request.headers.set("Authorization", `Bearer ${token}`);
      request.headers.set("Accept-Language", opts.locale());
      request.headers.set("X-App-Version", opts.appVersion);
      return request;
    },
  };
  client.use(headers);
  return client;
}

/** TanStack Query bindings (useQuery/useMutation per operation) over the client. */
export function createApiQuery(client: ReturnType<typeof createApiClient>) {
  return createQueryClient(client);
}
