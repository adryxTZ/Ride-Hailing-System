/**
 * Build identity for one brand, shared by rider/app.config.ts and driver/app.config.ts.
 * Plain CommonJS on purpose: Expo transpiles only the app.config.ts file itself, so a
 * shared helper must be loadable by Node as is. Types: expo-brand.d.ts next to this file.
 *
 * Inputs (all chosen at build time, never named in code):
 *   BRAND    brand code: reads apps/brands/<BRAND>.json (names, bundle ids, scheme, API hosts, EAS ids)
 *   APP_ENV  local | uat | prod: picks api_base_url[APP_ENV]; API_BASE_URL overrides it
 *   apps/brands/<BRAND>/  exported build assets (icon, splash, theme.json) written by
 *            tools/brand-assets.mjs from the published theme; optional, so a fresh
 *            checkout still builds with Expo's defaults.
 *
 * Runtime brand data (live colours, logos, legal entity) still comes from GET /v1/app/config.
 */
const { existsSync, readFileSync } = require("node:fs");
const { join } = require("node:path");

/** Default brand code when BRAND is unset: the neutral development brand. */
const DEFAULT_BRAND = "nexa";
/** Default environment when APP_ENV is unset. */
const DEFAULT_ENV = "local";

/** readJSON parses a JSON file, or returns undefined when it does not exist. */
function readJSON(path) {
  return existsSync(path) ? JSON.parse(readFileSync(path, "utf8")) : undefined;
}

/** asset returns the absolute path of an exported asset when the file exists. */
function asset(dir, file) {
  const path = join(dir, file);
  return existsSync(path) ? path : undefined;
}

/** loadBrandBuild resolves the brand, app and environment from BRAND / APP_ENV / API_BASE_URL. */
function loadBrandBuild(kind) {
  const code = process.env.BRAND ?? DEFAULT_BRAND;
  const env = process.env.APP_ENV ?? DEFAULT_ENV;
  const brand = readJSON(join(__dirname, `${code}.json`));
  if (!brand) throw new Error(`apps/brands/${code}.json not found (BRAND=${code})`);
  const apiBaseUrl = process.env.API_BASE_URL ?? brand.api_base_url[env];
  if (!apiBaseUrl) throw new Error(`apps/brands/${code}.json has no api_base_url.${env}`);
  const assetsDir = join(__dirname, code);
  const theme = readJSON(join(assetsDir, "theme.json"));
  return {
    code,
    displayName: brand.display_name,
    scheme: brand.deep_link_scheme,
    app: brand[kind],
    env,
    apiBaseUrl,
    easOwner: brand.eas_owner,
    // A per-app icon (apps/brands/<code>/<app>/icon.png) wins over the brand-wide one.
    icon: asset(join(assetsDir, kind), "icon.png") ?? asset(assetsDir, "icon.png"),
    splash: asset(assetsDir, "splash.png"),
    primary: theme?.palette?.primary,
  };
}

/** Splash logo width in dp (the image is centred on the brand colour). */
const SPLASH_IMAGE_WIDTH = 200;

/** installed reports whether a package resolves from the app folder (cwd during expo config). */
function installed(name) {
  try {
    require.resolve(`${name}/package.json`, { paths: [process.cwd()] });
    return true;
  } catch {
    return false;
  }
}

/**
 * brandPlugins returns the config plugins that apply exported brand assets: the splash
 * screen (expo-splash-screen) when a splash was exported. Empty on a fresh checkout.
 */
function brandPlugins(b) {
  if (!b.splash) return [];
  if (!installed("expo-splash-screen")) {
    console.warn("expo-brand: splash exported but expo-splash-screen is not installed; run pnpm install");
    return [];
  }
  const splash = { image: b.splash, imageWidth: SPLASH_IMAGE_WIDTH, resizeMode: "contain" };
  return [["expo-splash-screen", b.primary ? { ...splash, backgroundColor: b.primary } : splash]];
}

/** easUpdatesURL is the EAS Update endpoint of a project (channels uat/prod per D11). */
function easUpdatesURL(projectId) {
  return `https://u.expo.dev/${projectId}`;
}

module.exports = { loadBrandBuild, brandPlugins, easUpdatesURL };
