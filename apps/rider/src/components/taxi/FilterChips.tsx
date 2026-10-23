/** FilterChips — Recommended / Faster / Cheaper (design 3, expanded sheet). */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";

export type OptionFilter = "recommended" | "faster" | "cheaper";

const CHIPS: { key: OptionFilter; label: string; icon?: string }[] = [
  { key: "recommended", label: "Recommended" },
  { key: "faster", label: "Faster", icon: "clock-time-four" },
  { key: "cheaper", label: "Cheaper", icon: "currency-usd" },
];

export function FilterChips({
  value,
  onChange,
}: {
  value: OptionFilter;
  onChange: (f: OptionFilter) => void;
}) {
  const { palette } = useTheme();
  return (
    <View style={styles.row}>
      {CHIPS.map((c) => {
        const on = c.key === value;
        return (
          <ScalePressable
            key={c.key}
            onPress={() => onChange(c.key)}
            style={[
              styles.chip,
              {
                backgroundColor: on ? palette.background : palette.surface,
                borderColor: on ? palette.primary : "transparent",
              },
            ]}
          >
            {c.icon ? <Icon name={c.icon} size={18} color={palette.text} /> : null}
            <Text style={styles.text}>{c.label}</Text>
          </ScalePressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", gap: 10 },
  chip: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    borderRadius: 10,
    borderWidth: 2,
    paddingHorizontal: 10,
    paddingVertical: 9,
  },
  text: { fontSize: 15 },
});
