/** Build-time constants for this app. Everything brand-related comes from the API at runtime. */

import type { AppKind } from "@platform/auth";
import Constants from "expo-constants";

export const APP_KIND: AppKind = "rider";
/** Rides (catalogue key): the app's main service. */
export const SERVICE_KEY = "taxi";
/** Package delivery by boda (catalogue key). */
export const PARCEL_SERVICE_KEY = "parcel";

export const APP_VERSION: string = Constants.expoConfig?.version ?? "0.0.0";

const CONFIGURED_API: string = Constants.expoConfig?.extra?.apiBaseUrl ?? "http://localhost:8080/v1";

/**
 * In development the API runs on the same machine as the dev server. When the
 * configured URL points at a loopback or LAN address, reuse the host the app was
 * loaded from (127.0.0.1 over USB with adb reverse, the PC's LAN IP over Wi‑Fi), so
 * one dev server serves phones on either link. Release builds use the URL as is.
 */
function resolveApiBaseUrl(): string {
  if (!__DEV__) return CONFIGURED_API;
  // On the web the page's own host is the dev machine (localhost, or its LAN IP from a phone browser).
  const webHost = typeof window !== "undefined" ? window.location?.hostname : undefined;
  const devHost = Constants.expoConfig?.hostUri?.split(":")[0] ?? webHost;
  if (!devHost) return CONFIGURED_API;
  // React Native's URL class lacks hostname/port getters, so parse by hand.
  const m = /^(https?:)\/\/([^/:]+)(?::(\d+))?(\/.*)?$/.exec(CONFIGURED_API);
  if (!m) return CONFIGURED_API;
  const [, protocol, host, port = "8080", path = ""] = m;
  const local = /^(localhost|127\.0\.0\.1|10\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.)/.test(host);
  if (!local) return CONFIGURED_API;
  return `${protocol}//${devHost}:${port}${path.replace(/\/$/, "")}`;
}

export const API_BASE_URL: string = resolveApiBaseUrl();
