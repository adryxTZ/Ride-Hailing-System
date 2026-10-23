/** Words for geofence kinds and statuses. */
import type { Tone } from "@/components/ui/Chip";

export const KIND_LABEL: Record<string, string> = { DISTRICT: "District", ZONE: "Zone", PLACE: "Place" };
export const KIND_HINT: Record<string, string> = {
  DISTRICT: "A large boundary, drawn or imported (e.g. a municipal district).",
  ZONE: "A drawn area such as a city centre or a toll road section.",
  PLACE: "A point and a radius, such as an airport, port or mall.",
};
export const STATUS: Record<string, { label: string; tone: Tone }> = {
  ACTIVE: { label: "Active", tone: "ok" },
  INACTIVE: { label: "Off", tone: "muted" },
  ARCHIVED: { label: "Archived", tone: "muted" },
};

/** "2.4 km²" or "850 m²". */
export function areaText(m2: number): string {
  return m2 >= 1_000_000
    ? `${(m2 / 1_000_000).toLocaleString("en-US", { maximumFractionDigits: 1 })} km²`
    : `${Math.round(m2).toLocaleString("en-US")} m²`;
}
