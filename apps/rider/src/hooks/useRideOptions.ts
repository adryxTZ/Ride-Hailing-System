/**
 * useRideOptions — fetches the quote for the booking draft once per route change
 * and turns it into display rows: name, picture, live pickup ETA, seats, price.
 */
import { useEffect, useMemo, useRef } from "react";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useNearbyVehicles } from "@/hooks/useNearbyVehicles";
import { useService } from "@/hooks/useServices";
import { useQuote } from "@/hooks/useTrips";
import { etaForClass } from "@/models/eta";
import { formatMoney } from "@/models/format";
import { filledStops, useBookingStore } from "@/services/bookingStore";
import { localized } from "@/services/catalog";

export type OptionSort = "recommended" | "faster" | "cheaper";

export interface RideOption {
  key: string;
  name: string;
  icon?: string;
  description?: string;
  seats?: number;
  etaMin?: number;
  fare: number;
  price: string;
  available: boolean;
}

export function useRideOptions(sort: OptionSort) {
  const booking = useBookingStore();
  const { data: config } = useAppConfig();
  const { service } = useService(booking.serviceKey ?? "");
  const { data: cars } = useNearbyVehicles(booking.pickup?.point, booking.serviceKey);
  const quote = useQuote();
  const locale = config?.brand.locale ?? "en";
  const inFlight = useRef(false);

  useEffect(() => {
    if (!booking.serviceKey || !booking.pickup || !booking.dropoff || booking.quote || inFlight.current)
      return;
    inFlight.current = true;
    quote.mutate(
      {
        service: booking.serviceKey,
        pickup: booking.pickup.point,
        waypoints: filledStops(booking.stops).map((st) => st.point),
        dropoff: booking.dropoff.point,
        attributes: booking.attributes,
      },
      {
        onSuccess: (q) => {
          booking.setQuote(q);
          const preferred = q.options.find((o) => o.vehicle_class === booking.vehicleClass && o.available);
          const first = preferred ?? q.options.find((o) => o.available);
          if (first) booking.setVehicleClass(first.vehicle_class);
        },
        onSettled: () => {
          inFlight.current = false;
        },
      },
    );
  }, [booking, quote]);

  const options = useMemo<RideOption[]>(() => {
    const classes = new Map((service?.vehicle_classes ?? []).map((c) => [c.key, c]));
    const currency = booking.quote?.options[0]?.currency ?? config?.brand.currency ?? "";
    const rows = (booking.quote?.options ?? []).map((o) => {
      const c = classes.get(o.vehicle_class);
      const fare = Number(o.fare ?? 0);
      return {
        key: o.vehicle_class,
        name: c ? localized(c.name, locale) : o.vehicle_class,
        icon: c?.icon,
        description: c?.description ? localized(c.description, locale) : undefined,
        seats: c?.seats,
        etaMin: etaForClass(cars, o.vehicle_class),
        fare,
        price: `${o.currency ?? currency} ${formatMoney(fare)}`,
        available: o.available,
      };
    });
    if (sort === "cheaper") rows.sort((a, b) => a.fare - b.fare);
    if (sort === "faster") rows.sort((a, b) => (a.etaMin ?? Infinity) - (b.etaMin ?? Infinity));
    return rows;
  }, [booking.quote, service, cars, sort, locale, config?.brand.currency]);

  const recommendedKey = booking.quote?.options.find((o) => o.available)?.vehicle_class;
  return { options, recommendedKey, quote: booking.quote, pending: quote.isPending, error: quote.error };
}
