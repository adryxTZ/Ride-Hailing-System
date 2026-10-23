/** VehicleCard — the registered vehicle: its ride type and number plate. */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";

export function VehicleCard({ label, plate }: { label: string; plate?: string }) {
  const { palette } = useTheme();
  const warn = palette.warning ?? palette.accent;
  return (
    <View style={[styles.card, { backgroundColor: palette.surface, borderColor: palette.border }]}>
      <View style={[styles.icon, { backgroundColor: withAlpha(palette.primary, 0.12) }]}>
        <Icon name="car-side" size={26} color={palette.primary} />
      </View>
      <View style={styles.body}>
        <Text style={styles.title}>{label}</Text>
        <Text variant="muted">Your vehicle</Text>
      </View>
      <View style={[styles.plate, { backgroundColor: withAlpha(warn, plate ? 0.9 : 0.18) }]}>
        <Text style={styles.plateText}>{plate ?? "No plate yet"}</Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    borderRadius: 18,
    borderWidth: 1,
    padding: 12,
  },
  icon: { width: 48, height: 48, borderRadius: 14, alignItems: "center", justifyContent: "center" },
  body: { flex: 1, gap: 2 },
  title: { fontSize: 16, fontWeight: "700" },
  plate: { borderRadius: 8, paddingHorizontal: 8, paddingVertical: 4 },
  plateText: { fontSize: 12, fontWeight: "700" },
});
