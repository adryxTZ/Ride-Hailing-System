/** Build-time settings. Everything brand-related comes from GET /v1/app/config at runtime. */
export const API_BASE_URL: string = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/v1";

/** The API's origin, for links it returns relative ("/v1/files/…"). */
export const API_ORIGIN: string = API_BASE_URL.replace(/\/v1\/?$/, "");

/** Absolute URL for a path the API returned relative to its origin. */
export const apiUrl = (path: string) => (path.startsWith("/") ? API_ORIGIN + path : path);

export const APP_VERSION = "0.1.0";
