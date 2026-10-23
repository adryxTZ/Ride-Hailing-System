/**
 * DriverCard — design 6: "Pickup in 2 min", the vehicle line, the driver's name
 * and photo, then "Message your driver" (unread badge) with a call button.
 */
import { Avatar, Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { Linking, StyleSheet, View } from "react-native";
import { FareRow } from "./FareRow";
import { SheetCard } from "./SheetCard";

interface Props {
  headline: string;
  value: string;
  vehicle: string;
  driverName?: string | null;
  driverPhone?: string;
  /** "TZS 6,658" and how it is paid; shown on the card at every stage. */
  fare?: { amount: string; method?: string };
  /** Opens the chat with the driver. */
  onMessage: () => void;
  /** Driver messages not read yet (badge on the message button). */
  unread?: number;
}

export function DriverCard({
  headline,
  value,
  vehicle,
  driverName,
  driverPhone,
  fare,
  onMessage,
  unread = 0,
}: Props) {
  const { palette } = useTheme();
  return (
    <SheetCard>
      <View style={styles.top}>
        <View style={styles.flex}>
          <Text style={styles.headline}>{headline}</Text>
          <Text style={styles.value}>{value}</Text>
        </View>
        <Avatar name={driverName ?? undefined} size={64} />
      </View>
      <Text style={styles.vehicle}>{vehicle}</Text>
      {driverName ? <Text variant="muted">{driverName}</Text> : null}
      {fare ? <FareRow amount={fare.amount} method={fare.method} /> : null}
      <View style={styles.actions}>
        <ScalePressable
          accessibilityLabel="Message driver"
          onPress={onMessage}
          style={[styles.notes, { backgroundColor: palette.surface }]}
        >
          <Icon name="message-processing" size={22} color={palette.text} />
          <Text variant="muted" style={styles.flex}>
            {unread > 0 ? "New message from your driver" : "Message your driver"}
          </Text>
          {unread > 0 ? (
            <View style={[styles.badge, { backgroundColor: palette.primary }]}>
              <Text style={[styles.badgeText, { color: palette.on_primary ?? palette.background }]}>
                {unread}
              </Text>
            </View>
          ) : null}
        </ScalePressable>
        {driverPhone ? (
          <ScalePressable
            accessibilityLabel="Call driver"
            onPress={() => void Linking.openURL(`tel:${driverPhone}`)}
            style={[styles.call, { backgroundColor: palette.surface }]}
          >
            <Icon name="phone" size={22} color={palette.text} />
          </ScalePressable>
        ) : null}
      </View>
    </SheetCard>
  );
}

const styles = StyleSheet.create({
  top: { flexDirection: "row", alignItems: "center", gap: 12 },
  flex: { flex: 1 },
  headline: { fontSize: 15 },
  value: { fontSize: 28, fontWeight: "700" },
  vehicle: { fontSize: 16, fontWeight: "700", marginTop: 6 },
  actions: { flexDirection: "row", gap: 12, marginTop: 8 },
  notes: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    borderRadius: 28,
    paddingHorizontal: 16,
    height: 56,
  },
  badge: {
    minWidth: 22,
    height: 22,
    borderRadius: 11,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 6,
  },
  badgeText: { fontSize: 12, fontWeight: "700" },
  call: { width: 56, height: 56, borderRadius: 28, alignItems: "center", justifyContent: "center" },
});
