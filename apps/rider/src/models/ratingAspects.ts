/** The chips a rider can add to a driver rating (keys are stored; labels are shown). */
import type { RatingAspect } from "@platform/chat";

export const DRIVER_PRAISE: RatingAspect[] = [
  { key: "friendly", label: "Friendly" },
  { key: "safe_driving", label: "Safe driving" },
  { key: "clean_vehicle", label: "Clean vehicle" },
  { key: "on_time", label: "On time" },
  { key: "knows_route", label: "Knew the route" },
];

export const DRIVER_PROBLEMS: RatingAspect[] = [
  { key: "late", label: "Late" },
  { key: "unsafe_driving", label: "Unsafe driving" },
  { key: "rude", label: "Rude" },
  { key: "dirty_vehicle", label: "Dirty vehicle" },
  { key: "wrong_route", label: "Wrong route" },
  { key: "overcharged", label: "Asked for more money" },
];
