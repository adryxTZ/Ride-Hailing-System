/**
 * navigation — turns road-route manoeuvres into what the driver reads:
 * an arrow icon, a sentence ("Turn left onto Makongo Road") and the distance
 * to the manoeuvre. Pure functions; the step data comes from GET /routes.
 */
import { distanceKm } from "@/models/geo";
import type { RouteStep } from "@/services/routes";

type LatLng = { lat: number; lng: number };

/** Within this distance a manoeuvre counts as reached; the banner moves to the next one. */
export const REACHED_M = 25;
const METRES_PER_KM = 1000;
const ROUND_SHORT_M = 10;
const ROUND_LONG_M = 50;
const SHORT_LIMIT_M = 100;

export interface Instruction {
  icon: string;
  text: string;
}

const TURN_ICON: Record<string, string> = {
  left: "arrow-left-top",
  right: "arrow-right-top",
  "sharp left": "arrow-left-bottom",
  "sharp right": "arrow-right-bottom",
  "slight left": "arrow-top-left",
  "slight right": "arrow-top-right",
  straight: "arrow-up",
  uturn: "arrow-u-left-top",
};

const ordinal = (n: number) => `${n}${n === 1 ? "st" : n === 2 ? "nd" : n === 3 ? "rd" : "th"}`;
const onto = (street?: string) => (street ? ` onto ${street}` : "");

/** The icon and sentence for one manoeuvre. */
export function instructionFor(step: RouteStep): Instruction {
  const mod = step.modifier ?? "straight";
  const icon = TURN_ICON[mod] ?? "arrow-up";
  const dir = mod === "uturn" ? "Make a U-turn" : mod === "straight" ? "Go straight" : `Turn ${mod}`;
  switch (step.type) {
    case "depart":
      return { icon: "navigation-variant", text: `Head off${onto(step.street)}` };
    case "arrive":
      return { icon: "flag-checkered", text: "You have arrived" };
    case "waypoint":
      return { icon: "map-marker", text: "Arrive at the stop" };
    case "roundabout":
    case "rotary":
    case "exit roundabout":
    case "exit rotary":
      return {
        icon: "rotate-right",
        text: step.exit
          ? `At the roundabout, take the ${ordinal(step.exit)} exit${onto(step.street)}`
          : `Go through the roundabout${onto(step.street)}`,
      };
    case "fork":
      return { icon, text: `Keep ${mod.replace("slight ", "")}${onto(step.street)}` };
    case "merge":
      return { icon, text: `Merge${onto(step.street)}` };
    case "on ramp":
    case "off ramp":
      return { icon, text: `Take the ramp${onto(step.street)}` };
    case "continue":
    case "new name":
      return mod === "straight" || !step.modifier
        ? { icon: "arrow-up", text: `Continue${onto(step.street)}` }
        : { icon, text: `${dir}${onto(step.street)}` };
    default: // turn, end of road, notification…
      return { icon, text: `${dir}${onto(step.street)}` };
  }
}

/** Metres between two points. */
export function metresBetween(a: LatLng, b: LatLng): number {
  return distanceKm(a, b) * METRES_PER_KM;
}

/** "80 m", "350 m", "1.4 km". */
export function formatDistance(m: number): string {
  if (m >= METRES_PER_KM) return `${(m / METRES_PER_KM).toFixed(1)} km`;
  const unit = m < SHORT_LIMIT_M ? ROUND_SHORT_M : ROUND_LONG_M;
  return `${Math.max(unit, Math.round(m / unit) * unit)} m`;
}

/**
 * The index of the manoeuvre the driver is heading to: the first after the
 * start (step 0 is "depart" from where the route was asked) that is not
 * already passed. `passed` is the highest index the driver has reached.
 */
export function nextStepIndex(steps: RouteStep[], passed: number): number {
  return Math.min(Math.max(1, passed + 1), steps.length - 1);
}
