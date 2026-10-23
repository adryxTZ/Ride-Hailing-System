import type { Trip } from "@/services/trips";
import { waitingState } from "./waiting";

const arrived = Date.UTC(2026, 9, 20, 9, 0, 0);
const trip = {
  timestamps: { arrived_pickup_at: new Date(arrived).toISOString() },
  quote_option: { waiting_grace_s: 180, waiting_rate_per_min: "130", waiting_cap: "450" },
} as unknown as Trip;

describe("waitingState", () => {
  it("counts down the free minutes", () => {
    const w = waitingState(trip, arrived + 70_000);
    expect(w?.freeLeftS).toBe(110);
    expect(w?.charging).toBe(false);
  });
  it("charges started minutes after the free time, capped", () => {
    expect(waitingState(trip, arrived + 4 * 60_000 + 1_000)?.fee).toBe(260);
    expect(waitingState(trip, arrived + 20 * 60_000)?.fee).toBe(450);
  });
  it("is undefined before the driver arrives", () => {
    expect(waitingState({ timestamps: {} } as unknown as Trip, arrived)).toBeUndefined();
  });
});
