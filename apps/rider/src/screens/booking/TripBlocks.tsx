/** The phase-specific top block of the rider's trip sheet, and the payment card. */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { useRouter } from "expo-router";
import { Alert, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { ReceiptActions } from "@/components/activity/ReceiptActions";
import { TripRating } from "@/components/rating/TripRating";
import { DriverCard } from "@/components/taxi/DriverCard";
import { FareBreakdown, type FareLineView } from "@/components/taxi/FareBreakdown";
import { FarePaymentCard } from "@/components/taxi/FarePaymentCard";
import { FareRow } from "@/components/taxi/FareRow";
import { OutcomeCard } from "@/components/taxi/OutcomeCard";
import { PaymentSummaryCard } from "@/components/taxi/PaymentSummaryCard";
import { SearchingBar } from "@/components/taxi/SearchingBar";
import { SheetCard } from "@/components/taxi/SheetCard";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useUnreadCount } from "@/hooks/useChat";
import { useFarePayment } from "@/hooks/useFarePayment";
import { useNow } from "@/hooks/useNow";
import { useOpenReceiptPdf, useReceipt } from "@/hooks/useReceipt";
import { useService } from "@/hooks/useServices";
import { etaMinutes } from "@/models/eta";
import { fareView } from "@/models/farePayment";
import { clockIn, formatMoney } from "@/models/format";
import { distanceKm } from "@/models/geo";
import { receiptLines } from "@/models/receipt";
import { mmss, waitingState } from "@/models/waiting";
import { useBookingStore } from "@/services/bookingStore";
import { localized } from "@/services/catalog";
import type { Point, Trip } from "@/services/trips";
import type { TripPhase } from "./tripPhase";

interface Props {
  trip?: Trip;
  phase: TripPhase;
  looking: boolean;
  /** Road travel time from the driver to the stop being driven to (seconds). */
  etaS?: number;
  onCancel: () => void;
}

export function TripStatusBlock({ trip, phase, looking, etaS, onCancel }: Props) {
  const vehicle = useVehicleLabel(trip);
  const fare = `${trip?.currency ?? ""} ${formatMoney(trip?.final_fare ?? trip?.estimated_fare)}`;
  const outcome = useOutcomeActions();
  const router = useRouter();
  const unread = useUnreadCount(trip?.id, "RIDER");
  const message = () => trip && router.push({ pathname: "/chat/[id]", params: { id: trip.id } });
  const driverAt = trip?.driver_location?.location;
  const pickup = trip?.stops.find((s) => s.kind === "PICKUP")?.location;
  const dropoff = trip?.stops.find((s) => s.kind === "DROPOFF")?.location;
  const card = (headline: string, value: string) => (
    <DriverCard
      headline={headline}
      value={value}
      vehicle={vehicle}
      driverName={trip?.driver?.name}
      driverPhone={trip?.driver?.phone}
      fare={{ amount: fare, method: trip?.payment_method }}
      onMessage={message}
      unread={unread}
    />
  );
  const lines = receiptLines(trip);
  const now = useNow(phase === "arrived");
  const waiting = trip ? waitingState(trip, now) : undefined;
  const receipt = (
    <FareBreakdown
      lines={lines}
      total={trip?.final_fare ?? trip?.estimated_fare ?? 0}
      currency={trip?.currency ?? ""}
      fees={trip?.fees}
    />
  );

  switch (phase) {
    case "assigned":
      return card(
        "Pickup in",
        etaS != null
          ? `${Math.max(1, Math.round(etaS / 60))} min`
          : (minutesBetween(driverAt, pickup) ?? "On the way"),
      );
    case "arrived":
      return card(
        "Your driver has arrived",
        waiting?.charging
          ? `Waiting fee ${trip?.currency ?? ""} ${formatMoney(waiting.fee)}`
          : waiting
            ? `Free wait ${mmss(waiting.freeLeftS)}`
            : "Meet at pickup",
      );
    case "riding":
      if (trip?.status === "ARRIVED_AT_DROPOFF") return card("You have arrived", "At your destination");
      return card(
        "Arriving at",
        etaS != null ? clockIn(etaS) : (arrivalClock(driverAt, dropoff) ?? "On the way"),
      );
    case "paying":
      return trip ? (
        <FareBlock trip={trip} fare={fare}>
          {receipt}
        </FareBlock>
      ) : null;
    case "done":
      return (
        <OutcomeCard
          icon="check-circle"
          title="Thanks for riding"
          body={
            trip?.payment?.status === "COLLECTED"
              ? `You paid ${fare} by mobile money.`
              : `You paid ${fare} in cash.`
          }
          action="Done"
          onAction={outcome.finish}
        >
          {trip ? <TripRating tripId={trip.id} driverName={trip.driver?.name} /> : null}
          {receipt}
          {trip ? <DoneReceipt tripId={trip.id} /> : null}
        </OutcomeCard>
      );
    case "no_show":
      return (
        <OutcomeCard
          icon="account-clock"
          title="You missed your ride"
          body={`Your driver waited at the pickup and had to leave. A no-show fee of ${trip?.currency ?? ""} ${formatMoney(trip?.cancellation_fee)} will be added to your next ride.`}
          action="Book again"
          onAction={outcome.retry}
          secondary={{ label: "Back to home", onPress: outcome.finish }}
        />
      );
    case "no_driver":
      return (
        <OutcomeCard
          icon="car-off"
          title="No drivers available"
          body="All drivers nearby are busy right now. Try again in a moment."
          action="Try again"
          onAction={outcome.retry}
          secondary={{ label: "Back to home", onPress: outcome.finish }}
        />
      );
    default:
      return (
        <SearchingCard looking={looking} onCancel={onCancel} fare={fare} method={trip?.payment_method} />
      );
  }
}

/** The receipt number and PDF download on the "Thanks for riding" card. */
function DoneReceipt({ tripId }: { tripId: string }) {
  const { data } = useReceipt(tripId, true);
  const pdf = useOpenReceiptPdf(tripId);
  return (
    <ReceiptActions
      number={data?.number}
      busy={pdf.isPending}
      error={pdf.error?.message}
      onOpen={(lang) => pdf.mutate(lang)}
    />
  );
}

/** The fare card while the ride's fare is owed, with its actions wired to the API. */
function FareBlock({ trip, fare, children }: { trip: Trip; fare: string; children: React.ReactNode }) {
  const view = fareView(trip, useNow(trip.payment?.status === "PENDING"));
  const { prompt, cash } = useFarePayment(trip.id);
  const error = (prompt.error ?? cash.error)?.message;
  return (
    <FarePaymentCard
      view={view}
      fare={fare}
      busy={prompt.isPending || cash.isPending}
      error={error}
      onRetry={(phone) => prompt.mutate(phone)}
      onCash={() => cash.mutate()}
    >
      {children}
    </FarePaymentCard>
  );
}

function SearchingCard({
  looking,
  onCancel,
  fare,
  method,
}: {
  looking: boolean;
  onCancel: () => void;
  fare: string;
  method?: string;
}) {
  const { palette } = useTheme();
  return (
    <SheetCard style={styles.statusCard}>
      <View style={[styles.grabber, { backgroundColor: palette.border }]} />
      {looking ? (
        <Animated.View entering={FadeInDown} style={styles.block}>
          <Text style={styles.title}>Looking for a driver</Text>
          <Text variant="muted">Your booking is accepted. Finding you a driver…</Text>
        </Animated.View>
      ) : (
        <Animated.View entering={FadeInDown} style={styles.block}>
          <Text style={styles.title}>Ride requested</Text>
          <Text variant="muted">Finding drivers nearby</Text>
          <FareRow amount={fare} method={method} />
          <View style={styles.bar}>
            <SearchingBar />
          </View>
          <ScalePressable onPress={onCancel} style={styles.cancel}>
            <View style={[styles.cancelIcon, { backgroundColor: palette.surface }]}>
              <Icon name="car-off" size={26} color={palette.text} />
            </View>
            <Text style={styles.cancelText}>Cancel ride</Text>
          </ScalePressable>
        </Animated.View>
      )}
    </SheetCard>
  );
}

export function TripPayment({ trip }: { trip?: Trip | null }) {
  const vehicle = useVehicleLabel(trip ?? undefined, false);
  return (
    <PaymentSummaryCard
      category={vehicle || undefined}
      method={trip?.payment_method}
      amount={`${trip?.currency ?? ""} ${formatMoney(trip?.final_fare ?? trip?.estimated_fare)}`}
    />
  );
}

/** "Kawaida · Taxi" until vehicles carry a plate, colour and model. */
function useVehicleLabel(trip?: Trip, withService = true): string {
  const { data: config } = useAppConfig();
  const { service } = useService(trip?.service ?? "");
  const locale = config?.brand.locale ?? "en";
  const cls = service?.vehicle_classes.find((c) => c.key === trip?.vehicle_class);
  const className = cls ? localized(cls.name, locale) : "";
  return withService && service ? `${className} · ${localized(service.name, locale)}` : className;
}

function useOutcomeActions() {
  const router = useRouter();
  const reset = useBookingStore((s) => s.reset);
  const clearQuote = useBookingStore((s) => s.setQuote);
  const hasDraft = useBookingStore((s) => !!s.pickup && !!s.dropoff);
  return {
    finish: () => {
      reset();
      router.replace("/");
    },
    retry: () => {
      if (!hasDraft) return router.replace("/");
      clearQuote(undefined);
      router.replace("/book/options");
    },
  };
}

function minutesBetween(a?: Point, b?: Point): string | undefined {
  return a && b ? `${etaMinutes(distanceKm(a, b) * 1000)} min` : undefined;
}

function arrivalClock(a?: Point, b?: Point): string | undefined {
  return a && b ? clockIn(etaMinutes(distanceKm(a, b) * 1000) * 60) : undefined;
}

const styles = StyleSheet.create({
  statusCard: { borderTopLeftRadius: 24, borderTopRightRadius: 24, paddingTop: 10 },
  grabber: { alignSelf: "center", width: 40, height: 5, borderRadius: 3, marginBottom: 6 },
  block: { gap: 6 },
  title: { fontSize: 24, fontWeight: "700" },
  bar: { marginVertical: 14 },
  cancel: { alignSelf: "center", alignItems: "center", gap: 8 },
  cancelIcon: { width: 64, height: 64, borderRadius: 32, alignItems: "center", justifyContent: "center" },
  cancelText: { fontSize: 13 },
});
