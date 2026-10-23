/** WhereToBar — the big "Where to?" search bar with the "Later" chip (design 1). */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { Pressable, StyleSheet, View } from "react-native";

export function WhereToBar({ onPress, onLater }: { onPress: () => void; onLater: () => void }) {
  const { palette } = useTheme();
  return (
    <ScalePressable onPress={onPress} style={[styles.bar, { backgroundColor: palette.surface }]}>
      <Icon name="magnify" size={26} color={palette.text} />
      <Text style={styles.text}>Where to?</Text>
      <Pressable
        onPress={onLater}
        hitSlop={8}
        style={[styles.later, { backgroundColor: palette.background }]}
      >
        <Icon name="calendar-clock" size={18} color={palette.text} />
        <Text style={styles.laterText}>Later</Text>
      </Pressable>
      <View />
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: "row",
    alignItems: "center",
    gap: 14,
    borderRadius: 12,
    paddingHorizontal: 18,
    height: 64,
  },
  text: { flex: 1, fontSize: 19, fontWeight: "600" },
  later: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    borderRadius: 10,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  laterText: { fontSize: 14, fontWeight: "500" },
});
