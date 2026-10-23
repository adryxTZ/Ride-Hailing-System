import type { Trip } from "@/services/trips";
import { fareView } from "./farePayment";

const NOW = Date.parse("2026-10-23T10:00:00Z");

function trip(payment: Partial<NonNullable<Trip["payment"]>> | undefined): Trip {
  return {
    payment: payment && {
      method: "MOBILE_MONEY",
      amount: "8400",
      currency: "TZS",
      cash_chosen: false,
      status: "DUE",
      ...payment,
    },
  } as Trip;
}
const prompt = (p: object) => ({
  phone: "+255712000001",
  amount: "8400",
  expires_at: "2026-10-23T10:02:30Z",
  prompts_used: 1,
  prompts_left: 2,
  status: "PENDING",
  ...p,
});

describe("fareView", () => {
  it("asks the rider to approve an open prompt, with the time left", () => {
    expect(fareView(trip({ status: "PENDING", prompt: prompt({}) as never }), NOW)).toEqual({
      kind: "approve",
      phone: "+255712000001",
      secondsLeft: 150,
    });
  });
  it("explains a failure and how many tries are left", () => {
    const v = fareView(
      trip({
        status: "FAILED",
        prompt: prompt({ status: "FAILED", failure_code: "INSUFFICIENT_FUNDS", prompts_left: 1 }) as never,
      }),
      NOW,
    );
    expect(v).toEqual({
      kind: "failed",
      reason: "There was not enough money on that number.",
      promptsLeft: 1,
    });
  });
  it("shows cash for a cash ride, with mobile money as an option", () => {
    expect(fareView(trip({ method: "CASH" }), NOW)).toEqual({ kind: "cash", canUseMobileMoney: true });
  });
  it("shows cash only once the rider chose cash or ran out of prompts", () => {
    expect(
      fareView(
        trip({ status: "FAILED", cash_chosen: true, prompt: prompt({ status: "FAILED" }) as never }),
        NOW,
      ),
    ).toEqual({ kind: "cash", canUseMobileMoney: false });
    expect(
      fareView(trip({ status: "DUE", prompt: prompt({ status: "FAILED", prompts_left: 0 }) as never }), NOW)
        .kind,
    ).toBe("cash");
  });
  it("is paid once collected", () => {
    expect(fareView(trip({ status: "COLLECTED" }), NOW)).toEqual({ kind: "paid" });
  });
});
