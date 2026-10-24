/**
 * @platform/privacy — data-protection requests shared by the rider and driver apps.
 *
 *   model.ts               request types and display rules (labels, tones, date lines)
 *   service.ts             list / create / cancel / download-link over the app's client
 *   cardStyle.ts           card geometry and entrance timing
 *   PrivacyActionTile.tsx  "Download my data" / "Delete my account" cards
 *   DeleteConfirmCard.tsx  inline confirmation before a deletion request
 *   PrivacyRequestCard.tsx one request with its status and action
 *
 * Presentational and data access only; each app owns its screen and query hooks.
 */
export * from "./DeleteConfirmCard";
export * from "./model";
export * from "./PrivacyActionTile";
export * from "./PrivacyRequestCard";
export * from "./service";
