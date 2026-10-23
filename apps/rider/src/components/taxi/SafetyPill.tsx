/** SafetyPill — the floating "Safety" button over the trip map (design 6). */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { type StyleProp, StyleSheet, type ViewStyle } from "react-native";

export function SafetyPill({ onPress, style }: { onPress: () => void; style?: StyleProp<ViewStyle> }) {
  const { palette } = useTheme();
  return (
    <ScalePressable onPress={onPress} style={[styles.pill, { backgroundColor: palette.background }, style]}>
      <Icon name="shield-half-full" size={22} color={palette.text} />
      <Text style={styles.text}>Safety</Text>
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  pill: {
    position: "absolute",
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    borderRadius: 26,
    paddingHorizontal: 18,
    height: 52,
    elevation: 4,
    shadowColor: "#000000",
    shadowOpacity: 0.12,
    shadowRadius: 10,
  },
  text: { fontSize: 16, fontWeight: "500" },
});
