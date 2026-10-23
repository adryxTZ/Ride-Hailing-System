/**
 * Expo configuration for the rider app.
 *
 * Everything brand-specific (display name, slug, bundle ids, scheme, icon, splash plugin,
 * EAS project) is resolved by ../brands/expo-brand.js from BRAND, APP_ENV and the
 * exported build assets in apps/brands/<BRAND>/. Nothing here names a product.
 * Store builds: eas.json profiles (development | preview | production).
 */

import type { ExpoConfig } from "expo/config";
import { brandPlugins, easUpdatesURL, loadBrandBuild } from "../brands/expo-brand.js";

const b = loadBrandBuild("rider");
const projectId = b.app.eas_project_id;

const config: ExpoConfig = {
  name: b.displayName,
  slug: b.app.slug,
  owner: b.easOwner,
  scheme: b.scheme,
  version: "0.1.0",
  orientation: "portrait",
  userInterfaceStyle: "automatic",
  icon: b.icon,
  ios: { bundleIdentifier: b.app.ios_bundle_id, supportsTablet: false },
  android: {
    package: b.app.android_package,
    adaptiveIcon: b.icon ? { foregroundImage: b.icon, backgroundColor: b.primary } : undefined,
  },
  plugins: [["expo-router", { root: "./src/app" }], "expo-status-bar", ...brandPlugins(b)],
  experiments: { typedRoutes: true },
  // Over-the-air JS updates per channel (uat/prod) once the EAS project exists.
  runtimeVersion: projectId ? { policy: "appVersion" } : undefined,
  updates: projectId ? { url: easUpdatesURL(projectId) } : undefined,
  extra: {
    brand: b.code,
    appEnv: b.env,
    apiBaseUrl: b.apiBaseUrl,
    eas: projectId ? { projectId } : undefined,
  },
};

export default config;
