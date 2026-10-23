/**
 * useNavigation — follows the driver along a route's manoeuvres. Reaching a
 * manoeuvre (within REACHED_M) advances to the next; a fresh route (the car
 * moved ~110 m and the route was re-asked from the new position) starts over.
 */
import { useEffect, useMemo, useState } from "react";
import {
  formatDistance,
  type Instruction,
  instructionFor,
  metresBetween,
  nextStepIndex,
  REACHED_M,
} from "@/models/navigation";
import type { RouteStep } from "@/services/routes";

type LatLng = { lat: number; lng: number };

export interface NavState {
  now: Instruction;
  distance: string;
  then?: Instruction;
}

export function useNavigation(steps: RouteStep[] | undefined, position: LatLng): NavState | undefined {
  const [passed, setPassed] = useState(0);
  useEffect(() => setPassed(0), [steps]); // new route from the current spot

  const idx = steps && steps.length > 1 ? nextStepIndex(steps, passed) : -1;
  const target = idx >= 0 ? steps?.[idx] : undefined;
  const metres = target ? metresBetween(position, target.location) : 0;

  useEffect(() => {
    if (target && metres <= REACHED_M && idx < (steps?.length ?? 0) - 1) setPassed(idx);
  }, [target, metres, idx, steps]);

  return useMemo(() => {
    if (!steps || !target) return undefined;
    const after = steps[idx + 1];
    return {
      now: instructionFor(target),
      distance: formatDistance(metres),
      then: after ? instructionFor(after) : undefined,
    };
  }, [steps, target, idx, metres]);
}
