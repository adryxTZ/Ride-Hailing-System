import type { Wallet } from "@/models/wallet";
import { cashOutState, type Payout, payoutText } from "./payout";

const wallet = (payouts: object | undefined) => ({ payouts }) as unknown as Wallet;
const on = { payouts_enabled: true, available_payout: "12500.40", payout_min: "5000", open_payout: null };

describe("cashOutState", () => {
  it("is off without a payout gateway", () => {
    expect(cashOutState(wallet({ ...on, payouts_enabled: false })).kind).toBe("off");
    expect(cashOutState(wallet(undefined)).kind).toBe("off");
  });
  it("offers whole shillings down to the minimum", () => {
    expect(cashOutState(wallet(on))).toEqual({ kind: "ready", available: 12500, min: 5000 });
    expect(cashOutState(wallet({ ...on, available_payout: "3000" }))).toEqual({
      kind: "below_min",
      available: 3000,
      min: 5000,
    });
    expect(cashOutState(wallet({ ...on, available_payout: "0" })).kind).toBe("nothing");
  });
  it("shows the open payout first", () => {
    const p = { id: "p", status: "REQUESTED" } as Payout;
    expect(cashOutState(wallet({ ...on, open_payout: p }))).toEqual({ kind: "open", payout: p });
  });
});

describe("payoutText", () => {
  it("explains each status", () => {
    expect(payoutText({ status: "REQUESTED" } as Payout)).toBe("Waiting for approval");
    expect(payoutText({ status: "REJECTED", decision_reason: "number unclear" } as Payout)).toBe(
      "Not approved: number unclear",
    );
    expect(payoutText({ status: "FAILED", failure_code: "BALANCE_CHANGED" } as Payout)).toMatch(
      /balance changed/,
    );
    expect(payoutText({ status: "FAILED", failure_code: "FLOAT_TOO_LOW" } as Payout)).toMatch(/unchanged/);
  });
});
