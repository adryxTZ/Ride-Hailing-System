import type { Trip } from "@/services/trips";
import { driverFareState } from "./farePayment";

const NOW = Date.parse("2026-10-23T10:00:00Z");
const trip = (payment: object, status = "AWAITING_PAYMENT") =>
  ({
    status,
    payment: {
      method: "MOBILE_MONEY",
      amount: "8400",
      currency: "TZS",
      cash_chosen: false,
      status: "DUE",
      ...payment,
    },
  }) as unknown as Trip;

describe("driverFareState", () => {
  it("waits while the rider approves the prompt", () => {
    const s = driverFareState(
      trip({ status: "PENDING", prompt: { expires_at: "2026-10-23T10:01:00Z" } }),
      NOW,
    );
    expect(s).toEqual({ kind: "waiting", secondsLeft: 60 });
  });
  it("lets the driver collect cash after a failed prompt", () => {
    expect(driverFareState(trip({ status: "FAILED" }), NOW).kind).toBe("collect");
  });
  it("says when the rider chose cash", () => {
    expect(driverFareState(trip({ status: "FAILED", cash_chosen: true }), NOW)).toEqual({
      kind: "collect",
      note: "The rider will pay you in cash.",
    });
  });
  it("collects cash on a cash ride", () => {
    expect(driverFareState(trip({ method: "CASH" }), NOW)).toEqual({
      kind: "collect",
      note: "Collect cash, then confirm below.",
    });
  });
  it("shows nothing before the ride ends", () => {
    expect(driverFareState(trip({}, "IN_TRANSIT"), NOW)).toEqual({ kind: "none" });
  });
});
