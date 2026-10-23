/** TripRow — one trip in Activity (design: history): vehicle picture, destination, when · status, price. */
import { ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { ServiceIcon } from "@/components/ServiceIcon";
import { destinationOf, paidText, statusWord, whenText } from "@/models/tripText";
import type { Trip } from "@/services/trips";

export function TripRow({
  trip,
  icon,
  onPress,
  divider = true,
}: {
  trip: Trip;
  icon?: string;
  onPress: () => void;
  divider?: boolean;
}) {
  const { palette } = useTheme();
  const status = statusWord(trip);
  return (
    <ScalePressable
      onPress={onPress}
      style={[
        styles.row,
        divider ? { borderBottomColor: palette.border, borderBottomWidth: StyleSheet.hairlineWidth } : null,
      ]}
    >
      <View style={[styles.pic, { backgroundColor: palette.surface }]}>
        <ServiceIcon name={icon} size={44} />
      </View>
      <View style={styles.body}>
        <Text style={styles.title} numberOfLines={2}>
          {destinationOf(trip)}
        </Text>
        <Text variant="muted">
          {status === "Completed"
            ? whenText(trip.requested_at)
            : `${whenText(trip.requested_at)} · ${status}`}
        </Text>
        <View style={[styles.chip, { backgroundColor: palette.surface }]}>
          <Text style={styles.chipText}>{trip.payment_method === "CASH" ? "Cash" : trip.payment_method}</Text>
        </View>
      </View>
      <Text style={styles.price}>{paidText(trip)}</Text>
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", gap: 14, paddingVertical: 16 },
  pic: { width: 72, height: 72, borderRadius: 12, alignItems: "center", justifyContent: "center" },
  body: { flex: 1, gap: 4 },
  title: { fontSize: 16 },
  chip: { alignSelf: "flex-start", borderRadius: 6, paddingHorizontal: 8, paddingVertical: 3, marginTop: 2 },
  chipText: { fontSize: 12, fontWeight: "600" },
  price: { fontSize: 15, fontWeight: "600" },
});
