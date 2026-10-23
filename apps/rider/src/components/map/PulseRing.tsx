/** PulseRing — an expanding, fading circle around the pickup while we look for drivers. */
import { useTheme } from "@platform/ui";
import { useEffect } from "react";
import { StyleSheet, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";

const SIZE = 240;
const WAVE_MS = 2200;

export function PulseRing() {
  const { palette } = useTheme();
  const t = useSharedValue(0);
  useEffect(() => {
    t.value = withRepeat(withTiming(1, { duration: WAVE_MS, easing: Easing.out(Easing.quad) }), -1, false);
  }, [t]);
  const wave = useAnimatedStyle(() => ({
    transform: [{ scale: 0.15 + t.value * 0.85 }],
    opacity: 0.35 * (1 - t.value),
  }));
  const still = { backgroundColor: palette.info ?? palette.primary };
  return (
    <View style={styles.box} pointerEvents="none">
      <Animated.View style={[styles.circle, still, wave]} />
      <View style={[styles.halo, still]} />
    </View>
  );
}

const styles = StyleSheet.create({
  box: { width: SIZE, height: SIZE, alignItems: "center", justifyContent: "center" },
  circle: { position: "absolute", width: SIZE, height: SIZE, borderRadius: SIZE / 2 },
  halo: { position: "absolute", width: SIZE * 0.55, height: SIZE * 0.55, borderRadius: SIZE, opacity: 0.12 },
});
