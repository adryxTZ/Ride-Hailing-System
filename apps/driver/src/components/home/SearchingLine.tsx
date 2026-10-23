/** SearchingLine — "Finding trips near you" with a softly pulsing live dot. */
import { Text, useTheme } from "@platform/ui";
import { useEffect } from "react";
import { StyleSheet, View } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withRepeat, withTiming } from "react-native-reanimated";

const PULSE_MS = 900;

export function SearchingLine({ text }: { text: string }) {
  const { palette } = useTheme();
  const t = useSharedValue(0);
  useEffect(() => {
    t.value = withRepeat(withTiming(1, { duration: PULSE_MS }), -1, true);
  }, [t]);
  const dot = useAnimatedStyle(() => ({
    opacity: 0.35 + t.value * 0.65,
    transform: [{ scale: 0.8 + t.value * 0.4 }],
  }));
  return (
    <View style={styles.row}>
      <Animated.View style={[styles.dot, { backgroundColor: palette.success ?? palette.primary }, dot]} />
      <Text style={[styles.text, { color: palette.success ?? palette.primary }]}>{text}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 8 },
  dot: { width: 8, height: 8, borderRadius: 4 },
  text: { fontSize: 14, fontWeight: "600" },
});
