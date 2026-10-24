import { authRoute } from "./authRoute";

describe("authRoute", () => {
  it("sends signed-out people to sign-in, wherever they are", () => {
    expect(authRoute(false, false, undefined)).toEqual({ kind: "sign-in" });
    expect(authRoute(false, true, "consent")).toEqual({ kind: "sign-in" });
    expect(authRoute(false, false, "(auth)")).toEqual({ kind: "stay" });
  });
  it("shows consent before anything else, replacing sign-in and stacking on app screens", () => {
    expect(authRoute(true, true, "(auth)")).toEqual({ kind: "consent", stack: false });
    expect(authRoute(true, true, "onboarding")).toEqual({ kind: "consent", stack: true });
    expect(authRoute(true, true, "consent")).toEqual({ kind: "stay" });
  });
  it("leaves sign-in and consent once nothing is owed", () => {
    expect(authRoute(true, false, "(auth)")).toEqual({ kind: "home", fromConsent: false });
    expect(authRoute(true, false, "consent")).toEqual({ kind: "home", fromConsent: true });
    expect(authRoute(true, false, "(tabs)")).toEqual({ kind: "stay" });
  });
});
