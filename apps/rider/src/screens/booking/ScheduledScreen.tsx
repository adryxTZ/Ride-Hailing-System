/** ScheduledScreen — "Your ride is scheduled": pickup time, route and fare, with Done and View upcoming. */
import { useTheme } from "@platform/ui";
import { useLocalSearchParams, useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { OutcomeCard } from "@/components/taxi/OutcomeCard";
import { useTrip } from "@/hooks/useTrips";
import { formatMoney } from "@/models/format";
import { pickupLabel } from "@/models/schedule";
import { useBookingStore } from "@/services/bookingStore";

export function ScheduledScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { palette } = useTheme();
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { data: trip } = useTrip(tripId);
  const reset = useBookingStore((s) => s.reset);
  const when = trip?.scheduled_for ? pickupLabel(trip.scheduled_for) : "";
  const to = trip?.stops.find((s) => s.kind === "DROPOFF")?.address ?? "your destination";
  const finish = (path: "/" | "/activity") => {
    reset();
    router.replace(path);
  };
  return (
    <View style={[styles.root, { backgroundColor: palette.surface, paddingBottom: insets.bottom + 16 }]}>
      <OutcomeCard
        icon="calendar-check"
        title="Your ride is scheduled"
        body={`Pickup ${when} to ${to}. ${trip ? `${trip.currency} ${formatMoney(trip.estimated_fare)}. ` : ""}We'll start finding your driver 15 minutes before and let you know.`}
        action="Done"
        onAction={() => finish("/")}
        secondary={{ label: "View upcoming rides", onPress: () => finish("/activity") }}
      />
    </View>
  );
}

const styles = StyleSheet.create({ root: { flex: 1, justifyContent: "flex-end" } });
