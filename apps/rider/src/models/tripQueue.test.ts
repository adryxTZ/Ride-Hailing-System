import { finishingEtaText, isFinishingDriver } from "./tripQueue";

describe("finishing driver", () => {
  it("is shown only for a queued trip", () => {
    expect(isFinishingDriver({ queued: true })).toBe(true);
    expect(isFinishingDriver({ queued: false })).toBe(false);
    expect(isFinishingDriver(undefined)).toBe(false);
  });
  it("rounds the estimate to minutes, at least one", () => {
    expect(finishingEtaText({ queued_pickup_eta_s: 427 })).toBe("Pickup in ~7 min");
    expect(finishingEtaText({ queued_pickup_eta_s: 20 })).toBe("Pickup in ~1 min");
    expect(finishingEtaText({})).toBe("Coming to you next");
  });
});
