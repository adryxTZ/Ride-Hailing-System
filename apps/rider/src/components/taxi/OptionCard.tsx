/** OptionCard — one vehicle option (design 3): picture, name, ETA · seats, one line, RECOMMENDED, price. */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { ServiceIcon } from "@/components/ServiceIcon";

interface Props {
  icon?: string;
  name: string;
  description?: string;
  etaMin?: number;
  seats?: number;
  price: string;
  selected: boolean;
  recommended?: boolean;
  unavailable?: boolean;
  onPress: () => void;
}

export function OptionCard(p: Props) {
  const { palette } = useTheme();
  const eta = p.unavailable ? "None nearby" : p.etaMin != null ? `${p.etaMin} min` : "No drivers nearby";
  return (
    <ScalePressable
      onPress={p.onPress}
      disabled={p.unavailable}
      style={[
        styles.card,
        { borderColor: p.selected ? palette.primary : "transparent" },
        p.unavailable ? styles.faded : null,
      ]}
    >
      <ServiceIcon name={p.icon} size={76} />
      <View style={styles.body}>
        <Text style={styles.name}>{p.name}</Text>
        <View style={styles.meta}>
          <Text variant="muted">{eta}</Text>
          {p.seats ? (
            <>
              <Icon name="account-outline" size={16} color={palette.muted} />
              <Text variant="muted">{p.seats}</Text>
            </>
          ) : null}
        </View>
        {p.description ? <Text variant="muted">{p.description}</Text> : null}
        {p.recommended ? (
          <View style={[styles.badge, { backgroundColor: palette.primary }]}>
            <Text style={[styles.badgeText, { color: palette.on_primary ?? palette.background }]}>
              RECOMMENDED
            </Text>
          </View>
        ) : null}
      </View>
      <Text style={styles.price}>{p.price}</Text>
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  card: {
    flexDirection: "row",
    alignItems: "center",
    gap: 14,
    padding: 14,
    borderRadius: 12,
    borderWidth: 2,
  },
  faded: { opacity: 0.4 },
  body: { flex: 1, gap: 3 },
  name: { fontSize: 18, fontWeight: "700" },
  meta: { flexDirection: "row", alignItems: "center", gap: 6 },
  badge: { alignSelf: "flex-start", borderRadius: 4, paddingHorizontal: 6, paddingVertical: 3, marginTop: 4 },
  badgeText: { fontSize: 11, fontWeight: "800", letterSpacing: 0.5 },
  price: { fontSize: 17, fontWeight: "700", alignSelf: "flex-start" },
});
