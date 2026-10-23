/** Words and tones for driver and document statuses. */
import type { Tone } from "@/components/ui/Chip";

export const DRIVER_STATUS: Record<string, { label: string; tone: Tone }> = {
  ONBOARDING: { label: "Signing up", tone: "muted" },
  PENDING_REVIEW: { label: "Waiting for review", tone: "warn" },
  APPROVED: { label: "Approved", tone: "ok" },
  REJECTED: { label: "Needs changes", tone: "bad" },
  SUSPENDED: { label: "Suspended", tone: "bad" },
};

export const DOC_STATUS: Record<string, { label: string; tone: Tone }> = {
  PENDING: { label: "To check", tone: "warn" },
  APPROVED: { label: "Approved", tone: "ok" },
  REJECTED: { label: "Rejected", tone: "bad" },
};

export const DOC_LABEL: Record<string, string> = {
  PHOTO: "Driver photo",
  LICENCE: "Driving licence",
  NATIONAL_ID: "National ID",
  REGISTRATION: "Registration card",
  INSURANCE: "Insurance",
  VEHICLE_PHOTO: "Vehicle photo",
};
