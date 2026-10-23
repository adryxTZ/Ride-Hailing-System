/** MoreActions — the "More" block on an active trip: share ride details, contact driver, cancel ride. */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { SheetCard } from "./SheetCard";

export interface MoreAction {
  icon: string;
  label: string;
  onPress: () => void;
}

export function MoreActions({ actions }: { actions: MoreAction[] }) {
  const { palette } = useTheme();
  return (
    <SheetCard>
      <Text style={styles.title}>More</Text>
      {actions.map((a) => (
        <ScalePressable key={a.label} onPress={a.onPress} style={styles.row}>
          <Icon name={a.icon} size={22} color={palette.text} />
          <Text style={styles.label}>{a.label}</Text>
          <View style={styles.flex} />
          <Icon name="chevron-right" size={22} color={palette.muted} />
        </ScalePressable>
      ))}
    </SheetCard>
  );
}

const styles = StyleSheet.create({
  title: { fontSize: 20, fontWeight: "700" },
  row: { flexDirection: "row", alignItems: "center", gap: 14, paddingVertical: 12 },
  label: { fontSize: 16 },
  flex: { flex: 1 },
});
