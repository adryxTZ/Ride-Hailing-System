/** useMyVehicle — the driver's registered vehicle as shown on Home and Account ("Standard · T123ABC"). */
import { useAppConfig } from "@/hooks/useAppConfig";
import { useOnboarding } from "@/hooks/useOnboarding";
import { localText, useTaxiClasses } from "@/hooks/useTaxiClasses";

export function useMyVehicle() {
  const { data: ob } = useOnboarding();
  const { classes } = useTaxiClasses();
  const { data: config } = useAppConfig();
  const v = ob?.vehicle;
  const className = v
    ? localText(classes.find((c) => c.key === v.vehicle_class)?.name, config?.brand.locale ?? "en")
    : "";
  const model = [v?.colour, v?.make, v?.model].filter(Boolean).join(" ");
  return {
    vehicle: v,
    status: ob?.profile.status,
    /** "Standard" — the ride type. */
    label: v ? className || v.vehicle_class : "No vehicle yet",
    /** "Silver Toyota IST". */
    model,
    plate: v?.plate,
  };
}
