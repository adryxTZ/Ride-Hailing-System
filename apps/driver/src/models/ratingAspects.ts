/** The chips a driver can add to a rider rating (keys are stored; labels are shown). */
import type { RatingAspect } from "@platform/chat";

export const RIDER_PRAISE: RatingAspect[] = [
  { key: "polite", label: "Polite" },
  { key: "on_time", label: "Ready on time" },
  { key: "clear_pickup", label: "Clear pickup point" },
  { key: "paid_promptly", label: "Paid promptly" },
];

export const RIDER_PROBLEMS: RatingAspect[] = [
  { key: "late", label: "Kept me waiting" },
  { key: "rude", label: "Rude" },
  { key: "wrong_pickup", label: "Wrong pickup point" },
  { key: "payment_issue", label: "Payment problem" },
  { key: "damaged_vehicle", label: "Damaged the vehicle" },
];
