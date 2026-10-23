/** ChoiceChip — a selectable pill (day or time) with the brand fill when chosen. */
import { ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet } from "react-native";

export function ChoiceChip({
  label,
  selected,
  onPress,
  wide,
}: {
  label: string;
  selected: boolean;
  onPress: () => void;
  wide?: boolean;
}) {
  const { palette } = useTheme();
  return (
    <ScalePressable
      accessibilityRole="button"
      accessibilityState={{ selected }}
      accessibilityLabel={label}
      onPress={onPress}
      style={[
        styles.chip,
        wide ? styles.wide : null,
        {
          backgroundColor: selected ? palette.primary : palette.surface,
          borderColor: selected ? palette.primary : palette.border,
        },
      ]}
    >
      <Text
        style={[styles.text, { color: selected ? (palette.on_primary ?? palette.background) : palette.text }]}
      >
        {label}
      </Text>
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  chip: {
    minHeight: 44,
    paddingHorizontal: 16,
    borderRadius: 22,
    borderWidth: 1,
    alignItems: "center",
    justifyContent: "center",
  },
  wide: { flexBasis: "30%", flexGrow: 1 },
  text: { fontSize: 15, fontWeight: "600" },
});
