/** onboarding model — what the driver reads about their registration. Pure functions. */
import type { DriverDocument, Onboarding, Requirement } from "@/services/onboarding";

export type DriverStatus = Onboarding["profile"]["status"];

/** Headline and explanation for each registration status. */
export function statusCopy(status: DriverStatus): { title: string; body: string; icon: string } {
  switch (status) {
    case "PENDING_REVIEW":
      return {
        icon: "clock-outline",
        title: "We're checking your documents",
        body: "This usually takes less than a day. We'll let you know as soon as you can drive.",
      };
    case "REJECTED":
      return {
        icon: "alert-circle-outline",
        title: "Some details need fixing",
        body: "See the note below, fix it, then send again.",
      };
    case "SUSPENDED":
      return {
        icon: "account-cancel-outline",
        title: "Your account is on hold",
        body: "Contact support to find out more.",
      };
    case "APPROVED":
      return { icon: "check-decagram", title: "You're approved", body: "Go online to start getting trips." };
    default:
      return {
        icon: "account-plus-outline",
        title: "Become a driver",
        body: "Add your details, your car and a few photos. It takes about 5 minutes.",
      };
  }
}

/** The current document for a requirement (the last upload of that kind). */
export function documentFor(ob: Onboarding, req: Requirement): DriverDocument | undefined {
  return ob.documents.find((d) => d.kind === req.kind);
}

/** "Waiting for review", "Approved", "Rejected: <reason>", or "Add photo". */
export function documentStatus(doc?: DriverDocument): {
  label: string;
  tone: "todo" | "pending" | "ok" | "bad";
} {
  if (!doc) return { label: "Add photo", tone: "todo" };
  if (doc.status === "APPROVED") return { label: "Approved", tone: "ok" };
  if (doc.status === "REJECTED")
    return { label: doc.status_reason ? `Retake: ${doc.status_reason}` : "Retake photo", tone: "bad" };
  return { label: "Uploaded", tone: "pending" };
}

/** How many of the checklist items are done (profile + vehicle + mandatory documents). */
export function progress(ob: Onboarding): { done: number; total: number } {
  const total = ob.requirements.filter((r) => r.mandatory).length + 2;
  return { done: total - ob.missing.length, total };
}
