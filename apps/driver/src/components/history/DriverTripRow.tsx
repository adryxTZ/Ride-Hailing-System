/** DriverTripRow — one past trip for the driver: when, pickup → drop-off, status, earnings and its receipt. */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { useReceiptPdf } from "@/hooks/useReceiptPdf";
import { earnedOn, money } from "@/models/earnings";
import type { Trip } from "@/services/trips";

const STATUS: Record<string, string> = {
  COMPLETED: "Completed",
  CANCELLED_BY_RIDER: "Rider cancelled",
  CANCELLED_BY_DRIVER: "You cancelled",
};

export function DriverTripRow({ trip }: { trip: Trip }) {
  const { palette } = useTheme();
  const at = (k: string) => trip.stops.find((s) => s.kind === k)?.address ?? "Pinned location";
  const done = trip.status === "COMPLETED";
  const pdf = useReceiptPdf();
  const d = new Date(trip.requested_at);
  const when = `${d.toLocaleDateString("en-GB", { day: "numeric", month: "short" })}, ${d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" })}`;
  return (
    <View style={[styles.row, { borderBottomColor: palette.border }]}>
      <View style={styles.body}>
        <Text variant="muted">{`${when} · ${STATUS[trip.status] ?? trip.status}`}</Text>
        <View style={styles.route}>
          <Icon name="circle-outline" size={12} color={palette.primary} />
          <Text numberOfLines={1} style={styles.flex}>
            {at("PICKUP")}
          </Text>
        </View>
        <View style={styles.route}>
          <Icon name="circle" size={12} color={palette.text} />
          <Text numberOfLines={1} style={styles.flex}>
            {at("DROPOFF")}
          </Text>
        </View>
      </View>
      <View style={styles.right}>
        <Text style={[styles.fare, { color: done ? palette.text : palette.muted }]}>
          {done ? money(earnedOn(trip), trip.currency) : "—"}
        </Text>
        {done ? (
          <ScalePressable
            accessibilityRole="button"
            accessibilityLabel="Open the receipt"
            onPress={() => pdf.mutate(trip.id)}
            disabled={pdf.isPending}
            style={styles.receipt}
          >
            <Icon name="receipt" size={16} color={palette.primary} />
            <Text style={[styles.receiptText, { color: palette.primary }]}>Receipt</Text>
          </ScalePressable>
        ) : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    paddingVertical: 14,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  body: { flex: 1, gap: 6 },
  route: { flexDirection: "row", alignItems: "center", gap: 8 },
  flex: { flex: 1 },
  fare: { fontSize: 16, fontWeight: "700" },
  right: { alignItems: "flex-end", gap: 6 },
  receipt: { flexDirection: "row", alignItems: "center", gap: 4, paddingVertical: 4 },
  receiptText: { fontSize: 13, fontWeight: "600" },
});
