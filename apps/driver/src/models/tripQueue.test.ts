import type { NextTrip } from "@/services/tripQueue";
import { isQueuedTrip, nextTripLine, nextTripPlace, showsNextTrip } from "./tripQueue";

const next = (over: Partial<NextTrip> = {}): NextTrip => ({
  trip_id: "b",
  code: "ABC123",
  pickup: { location: { lat: 0, lng: 0 }, address: "Kariakoo Market" },
  pickup_distance_m: 1240,
  ...over,
});

describe("trip queue", () => {
  it("recognises the queued next trip", () => {
    expect(isQueuedTrip({ queued: true })).toBe(true);
    expect(isQueuedTrip({ queued: false })).toBe(false);
    expect(isQueuedTrip({})).toBe(false);
  });
  it("shows the banner only while finishing", () => {
    expect(showsNextTrip({ status: "ARRIVED_AT_DROPOFF" })).toBe(true);
    expect(showsNextTrip({ status: "AWAITING_PAYMENT" })).toBe(true);
    expect(showsNextTrip({ status: "EN_ROUTE_TO_PICKUP" })).toBe(false);
    expect(showsNextTrip(undefined)).toBe(false);
  });
  it("writes who and how far", () => {
    expect(nextTripLine(next({ rider_first_name: "Asha" }))).toBe("Asha · 1.2 km away");
    expect(nextTripLine(next())).toBe("1.2 km away");
    expect(nextTripPlace(next({ pickup: { location: { lat: 0, lng: 0 } } }))).toBe("Pinned location");
  });
});
