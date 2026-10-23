/** StatsStrip — the dark stats card: online time plus acceptance, rating and cancellations. */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";

export interface Stat {
  icon: string;
  value: string;
  label: string;
}

export function StatsStrip({ stats }: { stats: Stat[] }) {
  const { palette } = useTheme();
  const ink = palette.background;
  return (
    <View style={[styles.card, { backgroundColor: palette.text }]}>
      {stats.map((s) => (
        <View key={s.label} style={styles.cell}>
          <View style={[styles.badge, { backgroundColor: withAlpha(ink, 0.14) }]}>
            <Icon name={s.icon} size={16} color={ink} />
          </View>
          <Text style={[styles.value, { color: ink }]}>{s.value}</Text>
          <Text style={[styles.label, { color: withAlpha(ink, 0.7) }]}>{s.label}</Text>
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  card: { flexDirection: "row", borderRadius: 18, paddingVertical: 16, paddingHorizontal: 8 },
  cell: { flex: 1, alignItems: "center", gap: 4 },
  badge: { width: 30, height: 30, borderRadius: 15, alignItems: "center", justifyContent: "center" },
  value: { fontSize: 18, fontWeight: "700" },
  label: { fontSize: 12 },
});
