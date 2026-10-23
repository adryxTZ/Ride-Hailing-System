import { Pressable, StyleSheet, View } from "react-native";
import { useTheme } from "../ThemeProvider";
import { Text } from "./Text";

/** RadioRow — a single-choice row with a trailing radio circle (cancel reasons, payment method). */
export function RadioRow({
  label,
  selected,
  onPress,
  subtitle,
}: {
  label: string;
  subtitle?: string;
  selected: boolean;
  onPress: () => void;
}) {
  const { palette } = useTheme();
  return (
    <Pressable onPress={onPress} style={styles.row}>
      <View style={{ flex: 1, gap: 2 }}>
        <Text>{label}</Text>
        {subtitle ? <Text variant="muted">{subtitle}</Text> : null}
      </View>
      <View style={[styles.outer, { borderColor: selected ? palette.primary : palette.muted }]}>
        {selected ? <View style={[styles.inner, { backgroundColor: palette.primary }]} /> : null}
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 16, minHeight: 56 },
  outer: {
    width: 24,
    height: 24,
    borderRadius: 12,
    borderWidth: 2,
    alignItems: "center",
    justifyContent: "center",
  },
  inner: { width: 12, height: 12, borderRadius: 6 },
});
