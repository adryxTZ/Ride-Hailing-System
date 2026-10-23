/** SearchingBar — the thin indeterminate progress bar under "Finding drivers nearby" (design 4). */
import { useTheme } from "@platform/ui";
import { useEffect } from "react";
import { type LayoutChangeEvent, StyleSheet, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";

const SEGMENT = 0.12; // share of the track the moving segment covers
const SWEEP_MS = 1600;

export function SearchingBar() {
  const { palette } = useTheme();
  const width = useSharedValue(0);
  const t = useSharedValue(0);
  useEffect(() => {
    t.value = withRepeat(withTiming(1, { duration: SWEEP_MS, easing: Easing.inOut(Easing.quad) }), -1, true);
  }, [t]);
  const seg = useAnimatedStyle(() => ({
    width: width.value * SEGMENT,
    transform: [{ translateX: t.value * width.value * (1 - SEGMENT) }],
  }));
  return (
    <View
      onLayout={(e: LayoutChangeEvent) => {
        width.value = e.nativeEvent.layout.width;
      }}
      style={[styles.track, { backgroundColor: palette.border }]}
    >
      <Animated.View style={[styles.seg, { backgroundColor: palette.primary }, seg]} />
    </View>
  );
}

const styles = StyleSheet.create({
  track: { height: 5, borderRadius: 3, overflow: "hidden", alignSelf: "stretch" },
  seg: { height: 5, borderRadius: 3 },
});
