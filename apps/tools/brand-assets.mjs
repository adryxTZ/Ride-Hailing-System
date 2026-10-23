#!/usr/bin/env node
// Export build assets for one brand (DOC/03 §5 "Build assets"): reads the published
// theme from a running backend (GET /v1/app/config) and writes what the store builds
// need into apps/brands/<code>/, which app.config.ts picks up:
//
//   apps/brands/<code>/icon.png        <- theme.assets.app_icon  (1024x1024 PNG)
//   apps/brands/<code>/splash.png      <- theme.assets.splash
//   apps/brands/<code>/logo_*.png ...  <- the other published slots (receipts, store listing)
//   apps/brands/<code>/theme.json      <- brand identity, palette, typography, theme version
//
// Usage (from apps/):
//   node tools/brand-assets.mjs                         BRAND or "nexa", API from brands/<code>.json (APP_ENV, default prod)
//   node tools/brand-assets.mjs --brand <code> --api https://uat-api.<domain>/v1
//   node tools/brand-assets.mjs --env local             use api_base_url.local from the brand file
//
// Run it after publishing a theme in the admin Branding page, commit the files, then
// build with EAS. Node 20+ only, no dependencies. Exit 1 on any failed download.
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

/** Folder holding <code>.json build identity and the exported <code>/ assets. */
const BRANDS_DIR = resolve(dirname(fileURLToPath(import.meta.url)), "../brands");
/** Slot names the backend publishes (backend/api/openapi.yaml, Theme.assets). */
const SLOTS = ["app_icon", "splash", "logo_full", "logo_mark", "logo_on_dark", "favicon"];
/** File names the Expo config expects for the two slots it consumes. */
const SLOT_FILE = { app_icon: "icon", splash: "splash" };
/** Store icon size required by Apple and Google. */
const STORE_ICON_PX = 1024;
/** Accepted image types (the backend refuses anything else at upload). */
const EXTENSIONS = { "image/png": "png", "image/webp": "webp", "image/jpeg": "jpg" };
const REQUEST_TIMEOUT_MS = 15_000;

/** parseArgs reads --brand, --api and --env, with BRAND / API_BASE_URL / APP_ENV fallbacks. */
function parseArgs(argv) {
  const out = { brand: process.env.BRAND ?? "nexa", api: process.env.API_BASE_URL, env: process.env.APP_ENV ?? "prod" };
  for (let i = 0; i < argv.length; i += 2) {
    const key = argv[i]?.replace(/^--/, "");
    if (!["brand", "api", "env"].includes(key) || argv[i + 1] === undefined) {
      throw new Error(`unknown or incomplete argument "${argv[i]}" (use --brand, --api, --env)`);
    }
    out[key] = argv[i + 1];
  }
  return out;
}

/** readBuildIdentity loads apps/brands/<code>.json (bundle ids, API hosts). */
async function readBuildIdentity(code) {
  const path = join(BRANDS_DIR, `${code}.json`);
  try {
    return JSON.parse(await readFile(path, "utf8"));
  } catch (err) {
    throw new Error(`cannot read ${path}: ${err.message}`);
  }
}

/** getJSON fetches and parses one JSON document, failing on non-2xx. */
async function getJSON(url) {
  const res = await fetch(url, { signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
  if (!res.ok) throw new Error(`GET ${url} -> ${res.status}`);
  return res.json();
}

/** download fetches one asset (following the API's redirect to the file) and returns bytes + extension. */
async function download(url) {
  const res = await fetch(url, { redirect: "follow", signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
  if (!res.ok) throw new Error(`GET ${url} -> ${res.status}`);
  const type = (res.headers.get("content-type") ?? "").split(";")[0].trim();
  const bytes = Buffer.from(await res.arrayBuffer());
  const ext = EXTENSIONS[type] ?? (isPNG(bytes) ? "png" : undefined);
  if (!ext) throw new Error(`GET ${url}: unexpected content type "${type}"`);
  return { bytes, ext };
}

/** isPNG checks the 8-byte PNG signature. */
function isPNG(bytes) {
  return bytes.length > 24 && bytes.readUInt32BE(0) === 0x89504e47 && bytes.readUInt32BE(4) === 0x0d0a1a0a;
}

/** pngSize reads width and height from the IHDR chunk (bytes 16..23). */
function pngSize(bytes) {
  return isPNG(bytes) ? { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) } : undefined;
}

/** checkIcon warns when the app icon would be rejected by the stores. */
function checkIcon(bytes, ext) {
  const size = pngSize(bytes);
  if (ext !== "png" || !size) return "app_icon should be a PNG (stores require PNG icons)";
  if (size.width !== size.height || size.width < STORE_ICON_PX) {
    return `app_icon is ${size.width}x${size.height}; stores need ${STORE_ICON_PX}x${STORE_ICON_PX}`;
  }
  return undefined;
}

/** exportAssets downloads every published slot into outDir and returns the files written. */
async function exportAssets(origin, assets, outDir) {
  const written = {};
  const warnings = [];
  for (const slot of SLOTS) {
    const path = assets?.[slot];
    if (!path) continue;
    const url = path.startsWith("http") ? path : origin + path;
    const { bytes, ext } = await download(url);
    if (slot === "app_icon") {
      const warning = checkIcon(bytes, ext);
      if (warning) warnings.push(warning);
    }
    const file = `${SLOT_FILE[slot] ?? slot}.${ext}`;
    await writeFile(join(outDir, file), bytes);
    written[slot] = file;
  }
  return { written, warnings };
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const identity = await readBuildIdentity(args.brand);
  const api = (args.api ?? identity.api_base_url?.[args.env])?.replace(/\/$/, "");
  if (!api) throw new Error(`no API URL: pass --api or set api_base_url.${args.env} in brands/${args.brand}.json`);
  const origin = new URL(api).origin; // asset paths are absolute paths on the API host (/v1/brand/assets/...)

  const config = await getJSON(`${api}/app/config`);
  if (config.brand?.code !== args.brand) {
    throw new Error(`${api} serves brand "${config.brand?.code}", not "${args.brand}"`);
  }
  const outDir = join(BRANDS_DIR, args.brand);
  await mkdir(outDir, { recursive: true });
  const { written, warnings } = await exportAssets(origin, config.theme?.assets, outDir);

  const theme = {
    exported_from: api,
    brand: config.brand,
    theme_version: config.theme?.version,
    preset: config.theme?.preset,
    palette: config.theme?.palette,
    typography: config.theme?.typography,
    files: written,
  };
  await writeFile(join(outDir, "theme.json"), `${JSON.stringify(theme, null, 2)}\n`);

  console.log(`brand ${args.brand}: theme v${theme.theme_version} from ${api}`);
  console.log(`wrote ${outDir}/theme.json${Object.values(written).map((f) => `, ${f}`).join("")}`);
  if (!written.app_icon) warnings.push("no app_icon published: builds use the Expo default icon");
  if (!written.splash) warnings.push("no splash published: builds use a plain splash in the primary colour");
  for (const w of warnings) console.warn(`warning: ${w}`);
}

main().catch((err) => {
  console.error(`brand-assets: ${err.message}`);
  process.exit(1);
});
