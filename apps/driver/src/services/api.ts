/**
 * api service — the single API client instance for this app (bearer header,
 * refresh-on-401) and the auth and consent use cases bound to it. Screens import from here,
 * never from the generated client directly.
 */
import { createSessionClient } from "@platform/auth";
import { API_BASE_URL, APP_KIND, APP_VERSION } from "@/config";

export const {
  client: api,
  auth,
  consent,
  freshAccessToken,
} = createSessionClient({
  baseUrl: API_BASE_URL,
  app: APP_KIND,
  appVersion: APP_VERSION,
  locale: () => "sw",
});
