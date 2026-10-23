import { type DriverDocument, expiryChip, expiryProblem, kindsText } from "./document";

const TODAY = new Date("2026-10-23T09:00:00Z");
const doc = (d: Partial<DriverDocument>) => ({ status: "APPROVED", ...d }) as DriverDocument;

describe("expiryChip", () => {
  it("says how long is left", () => {
    expect(expiryChip(doc({ expiry_state: "EXPIRING", days_left: 7 }))).toEqual({
      text: "Expires in 7 days",
      tone: "warn",
    });
    expect(expiryChip(doc({ expiry_state: "EXPIRED", days_left: -1 }))?.tone).toBe("bad");
  });
  it("shows a renewal under review first", () => {
    expect(
      expiryChip(doc({ expiry_state: "EXPIRED", renewal: { status: "PENDING" } as DriverDocument }))?.text,
    ).toBe("Renewal under review");
  });
  it("is silent for documents without expiry", () => {
    expect(expiryChip(doc({ expiry_state: "NOT_TRACKED" }))).toBeUndefined();
  });
});

describe("expiryProblem", () => {
  it("accepts a future date and refuses past, far and malformed ones", () => {
    expect(expiryProblem("2029-05-01", TODAY)).toBe("");
    expect(expiryProblem("2026-10-23", TODAY)).toBe("");
    expect(expiryProblem("2026-10-22", TODAY)).toMatch(/passed/);
    expect(expiryProblem("2040-01-01", TODAY)).toMatch(/10 years/);
    expect(expiryProblem("23/10/2029", TODAY)).toMatch(/YYYY-MM-DD/);
  });
});

describe("kindsText", () => {
  it("reads like a sentence", () => {
    expect(kindsText("INSURANCE, LICENCE")).toBe("insurance and licence");
    expect(kindsText("POLICE_CLEARANCE")).toBe("police clearance");
  });
});
