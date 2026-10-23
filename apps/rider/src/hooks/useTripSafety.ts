/** useTripSafety — the Safety sheet's state on a live trip: open/closed, share, SOS and their outcome. */
import { useState } from "react";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useShareTrip, useSos, useTrustedContacts } from "@/hooks/useSafety";
import type { Trip } from "@/services/trips";

export function useTripSafety(trip?: Trip | null) {
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState<string>();
  const { data: config } = useAppConfig();
  const { data: contacts } = useTrustedContacts();
  const shareM = useShareTrip();
  const sosM = useSos();
  const share = () => {
    if (!trip) return;
    shareM.mutate(
      { tripId: trip.id, driver: trip.driver?.name ?? undefined },
      { onError: (e) => setStatus(e.message) },
    );
  };
  const sos = () => {
    if (!trip) return;
    sosM.mutate(trip.id, {
      onSuccess: (a) =>
        setStatus(
          `Alert sent to ${a.contacts_alerted} contact${a.contacts_alerted === 1 ? "" : "s"}. Our team has been notified.`,
        ),
      onError: (e) => setStatus(e.message),
    });
  };
  return {
    open,
    setOpen: (v: boolean) => {
      setOpen(v);
      if (!v) setStatus(undefined);
    },
    status,
    share,
    sos,
    sharing: shareM.isPending,
    alerting: sosM.isPending,
    hasContacts: (contacts?.length ?? 0) > 0,
    emergencyPhone: (config?.brand as { emergency_phone?: string } | undefined)?.emergency_phone,
  };
}
