import { createApiClient } from "@platform/api-client";
import type { Middleware } from "openapi-fetch";
import { createAuthService } from "./authService";
import { type AppKind, useSessionStore } from "./session";

interface Options {
  baseUrl: string;
  app: AppKind;
  locale: () => string;
  appVersion: string;
}

const PROACTIVE_REFRESH_WINDOW_S = 60;
const REQUEST_TIMEOUT_MS = 15_000;

/**
 * Builds the API client for an app:
 *  - bearer header from the session (api-client middleware);
 *  - proactive refresh when the access token is within a minute of expiry;
 *  - one refresh-and-retry when the server answers 401, implemented as a fetch
 *    wrapper because openapi-fetch's onResponse hook requires a real Response
 *    instance, which React Native's fetch polyfill does not satisfy.
 * Returns the client and the auth service bound to it.
 */
export function createSessionClient(opts: Options) {
  let refreshing: Promise<unknown> | null = null;
  // auth is created after the client; the wrapper reads it lazily.
  let auth: ReturnType<typeof createAuthService> | undefined;
  const refreshOnce = () => {
    refreshing ??= (auth as ReturnType<typeof createAuthService>).refresh().finally(() => {
      refreshing = null;
    });
    return refreshing;
  };

  const fetchWithRetry: typeof globalThis.fetch = async (input, init) => {
    const request = input instanceof Request ? input : new Request(input, init);
    const retry = request.clone();
    // Fail fast on an unreachable host instead of hanging the screen.
    const response = await globalThis.fetch(request, { signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
    if (response.status !== 401 || isAuthRoute(request.url) || !useSessionStore.getState().session)
      return response;
    try {
      await refreshOnce();
    } catch {
      return response;
    }
    const fresh = useSessionStore.getState().session;
    if (!fresh) return response;
    retry.headers.set("Authorization", `Bearer ${fresh.accessToken}`);
    return globalThis.fetch(retry, { signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
  };

  const client = createApiClient({
    baseUrl: opts.baseUrl,
    locale: opts.locale,
    appVersion: opts.appVersion,
    getAccessToken: () => useSessionStore.getState().session?.accessToken ?? null,
    fetch: fetchWithRetry,
  });
  auth = createAuthService(client, opts.app);

  const proactiveRefresh: Middleware = {
    async onRequest({ request }) {
      const s = useSessionStore.getState().session;
      if (
        s &&
        s.expiresAt > 0 &&
        s.expiresAt - Date.now() / 1000 < PROACTIVE_REFRESH_WINDOW_S &&
        !isAuthRoute(request.url)
      ) {
        await refreshOnce().catch(() => undefined);
        const fresh = useSessionStore.getState().session;
        if (fresh) request.headers.set("Authorization", `Bearer ${fresh.accessToken}`);
      }
      return request;
    },
  };
  client.use(proactiveRefresh);

  /**
   * The access token for non-HTTP uses (the websocket), refreshed first when it
   * is about to expire. Goes through the same single-flight refresh as requests:
   * two parallel refreshes would reuse a rotated token, which the server treats
   * as theft and signs the user out.
   */
  const freshAccessToken = async (): Promise<string | undefined> => {
    const s = useSessionStore.getState().session;
    if (!s) return undefined;
    if (s.expiresAt === 0 || s.expiresAt - Date.now() / 1000 > PROACTIVE_REFRESH_WINDOW_S)
      return s.accessToken;
    await refreshOnce().catch(() => undefined);
    return useSessionStore.getState().session?.accessToken;
  };
  return { client, auth, freshAccessToken };
}

function isAuthRoute(url: string): boolean {
  return url.includes("/auth/");
}
