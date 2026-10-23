/** Types for expo-brand.js (build identity shared by the rider and driver app.config.ts). */

/** Which app is being configured. */
export type AppKind = "rider" | "driver";

/** Per-app identity inside apps/brands/<code>.json. */
export interface AppIdentity {
  slug: string;
  android_package: string;
  ios_bundle_id: string;
  /** EAS project id of this app for this brand (printed by `eas init`). */
  eas_project_id?: string;
}

/** Everything an app.config.ts needs, resolved for one brand, app and environment. */
export interface BrandBuild {
  code: string;
  displayName: string;
  scheme: string;
  app: AppIdentity;
  env: string;
  apiBaseUrl: string;
  /** Expo account (user or organisation) that owns the brand's EAS projects. */
  easOwner?: string;
  /** Absolute paths of exported assets, present only when exported. */
  icon?: string;
  splash?: string;
  /** Brand primary colour from the exported theme (splash and adaptive icon background). */
  primary?: string;
}

/** Resolves the brand, app and environment from BRAND / APP_ENV / API_BASE_URL. */
export function loadBrandBuild(kind: AppKind): BrandBuild;

/** Config plugins applying exported brand assets (splash screen); empty when none were exported. */
export function brandPlugins(b: BrandBuild): [string, Record<string, unknown>][];

/** The EAS Update endpoint of a project. */
export function easUpdatesURL(projectId: string): string;
