/** UpcomingCard — a ride booked ahead: pickup time, destination, ride type and fare, with Cancel. */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import { formatMoney } from "@/models/format";
import { pickupLabel } from "@/models/schedule";
import { destinationOf } from "@/models/tripText";
import type { Trip } from "@/services/trips";

interface Props {
  trip: Trip;
  className: string;
  cancelling: boolean;
  onCancel: () => void;
}

export function UpcomingCard({ trip, className, cancelling, onCancel }: Props) {
  const { palette } = useTheme();
  return (
    <View style={[styles.card, { backgroundColor: palette.surface, borderColor: palette.border }]}>
      <View style={[styles.badge, { backgroundColor: withAlpha(palette.primary, 0.12) }]}>
        <Icon name="calendar-clock" size={24} color={palette.primary} />
      </View>
      <View style={styles.body}>
        <Text style={styles.when}>{trip.scheduled_for ? pickupLabel(trip.scheduled_for) : ""}</Text>
        <Text numberOfLines={1}>{destinationOf(trip)}</Text>
        <Text variant="muted">{`${className} · ${trip.currency} ${formatMoney(trip.estimated_fare)}`}</Text>
      </View>
      <ScalePressable
        accessibilityLabel="Cancel scheduled ride"
        onPress={onCancel}
        disabled={cancelling}
        style={[styles.cancel, { borderColor: palette.border }]}
      >
        {cancelling ? (
          <ActivityIndicator color={palette.primary} />
        ) : (
          <Text style={styles.cancelText}>Cancel</Text>
        )}
      </ScalePressable>
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    borderWidth: 1,
    borderRadius: 18,
    padding: 14,
    shadowColor: "#000000",
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  badge: { width: 48, height: 48, borderRadius: 14, alignItems: "center", justifyContent: "center" },
  body: { flex: 1, gap: 2 },
  when: { fontSize: 17, fontWeight: "700" },
  cancel: {
    minHeight: 44,
    minWidth: 76,
    paddingHorizontal: 14,
    borderRadius: 22,
    borderWidth: 1,
    alignItems: "center",
    justifyContent: "center",
  },
  cancelText: { fontWeight: "600" },
});
