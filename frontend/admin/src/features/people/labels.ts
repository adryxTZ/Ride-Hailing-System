/** Words for payment states, devices and staff actions on a person. */
export const PAYMENT_LABEL: Record<string, string> = {
  DUE: "Due",
  PENDING: "Waiting for payment",
  FAILED: "Payment failed",
  COLLECTED: "Paid",
  CASH_CONFIRMED: "Cash received",
};

export const APP_LABEL: Record<string, string> = {
  rider: "Rider app",
  driver: "Driver app",
  admin: "Dashboard",
  RIDER: "Rider app",
  DRIVER: "Driver app",
};
