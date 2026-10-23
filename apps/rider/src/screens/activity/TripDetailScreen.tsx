/** TripDetailScreen — one past trip: map, when and status, driver and vehicle, route and the receipt. */
import { Header, Screen, Text, useTheme } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import { ReceiptActions } from "@/components/activity/ReceiptActions";
import { TaxiMap } from "@/components/map/TaxiMap";
import { FareBreakdown } from "@/components/taxi/FareBreakdown";
import { RouteSummaryCard } from "@/components/taxi/RouteSummaryCard";
import { useBack } from "@/hooks/useBack";
import { useOpenReceiptPdf, useReceipt } from "@/hooks/useReceipt";
import { useTrip } from "@/hooks/useTrips";
import { receiptLines } from "@/models/receipt";
import { paidText, statusWord, whenText } from "@/models/tripText";

const MAP_HEIGHT = 200;

export function TripDetailScreen({ id }: { id?: string }) {
  const back = useBack();
  const { palette } = useTheme();
  const { data: trip } = useTrip(id);
  const { data: receipt } = useReceipt(id, trip?.status === "COMPLETED");
  const pdf = useOpenReceiptPdf(id);
  if (!trip) {
    return (
      <Screen align="top">
        <Header title="Trip" leading="back" onLeading={() => back()} />
        <ActivityIndicator color={palette.primary} />
      </Screen>
    );
  }
  const at = (kind: string) => trip.stops.find((s) => s.kind === kind);
  const lines = receiptLines(trip);
  const completed = trip.status === "COMPLETED";
  return (
    <Screen align="top">
      <Header title={whenText(trip.requested_at)} leading="back" onLeading={() => back()} />
      <View style={[styles.map, { borderColor: palette.border }]} pointerEvents="none">
        <TaxiMap
          centre={at("PICKUP")?.location ?? { lat: -6.79, lng: 39.21 }}
          insets={{ top: 10, bottom: 10 }}
          pickup={at("PICKUP")?.location}
          dropoff={at("DROPOFF")?.location}
          stops={trip.stops.filter((s) => s.kind === "WAYPOINT").map((s) => s.location)}
          showsUser={false}
        />
      </View>
      <View style={styles.headRow}>
        <Text style={styles.status}>{statusWord(trip)}</Text>
        <Text style={styles.status}>{paidText(trip)}</Text>
      </View>
      <Text variant="muted">{`Trip ${trip.code}${trip.driver?.name ? ` · Driver ${trip.driver.name}` : ""}`}</Text>
      <RouteSummaryCard
        pickup={at("PICKUP")?.address ?? undefined}
        stops={trip.stops.filter((s) => s.kind === "WAYPOINT").map((s) => s.address ?? "Stop")}
        dropoff={at("DROPOFF")?.address ?? undefined}
      />
      {completed ? (
        <View style={[styles.receipt, { backgroundColor: palette.surface }]}>
          <Text style={styles.receiptTitle}>Receipt</Text>
          <FareBreakdown
            lines={lines}
            total={trip.final_fare ?? trip.estimated_fare}
            currency={trip.currency}
            fees={trip.fees}
          />
          <Text variant="muted">
            {trip.payment?.status === "COLLECTED" ? "Paid by mobile money" : "Paid in cash"}
          </Text>
          <ReceiptActions
            number={receipt?.number}
            busy={pdf.isPending}
            error={pdf.error?.message}
            onOpen={(lang) => pdf.mutate(lang)}
          />
        </View>
      ) : null}
    </Screen>
  );
}

const styles = StyleSheet.create({
  map: { height: MAP_HEIGHT, borderRadius: 16, overflow: "hidden", borderWidth: 1 },
  headRow: { flexDirection: "row", justifyContent: "space-between" },
  status: { fontSize: 18, fontWeight: "700" },
  receipt: { borderRadius: 16, padding: 16, gap: 12 },
  receiptTitle: { fontSize: 18, fontWeight: "700" },
});
