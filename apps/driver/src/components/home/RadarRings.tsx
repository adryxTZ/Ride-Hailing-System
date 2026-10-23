/**
 * RadarRings — dashed concentric rings around the driver with an expanding wave
 * while online ("we are looking for trips around you"). Offline they rest, muted.
 * Children (the car marker) render at the centre.
 */
import { useTheme, withAlpha } from "@platform/ui";
import { type PropsWithChildren, useEffect } from "react";
import { StyleSheet, View } from "react-native";
import Animated, {
  cancelAnimation,
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";

const RING_SIZES = [110, 190, 270];
const OUTER = RING_SIZES[RING_SIZES.length - 1];
const WAVE_MS = 2400;
const BREATH_MS = 1800;
const DISC = 84;

export function RadarRings({ active, children }: PropsWithChildren<{ active: boolean }>) {
  const { palette } = useTheme();
  const colour = active ? palette.primary : (palette.muted ?? palette.primary);
  const wave = useSharedValue(0);
  const breath = useSharedValue(0);

  useEffect(() => {
    if (active) {
      wave.value = withRepeat(
        withTiming(1, { duration: WAVE_MS, easing: Easing.out(Easing.quad) }),
        -1,
        false,
      );
      breath.value = withRepeat(
        withTiming(1, { duration: BREATH_MS, easing: Easing.inOut(Easing.sin) }),
        -1,
        true,
      );
    } else {
      cancelAnimation(wave);
      cancelAnimation(breath);
      wave.value = withTiming(0);
      breath.value = withTiming(0);
    }
  }, [active, wave, breath]);

  const waveStyle = useAnimatedStyle(() => ({
    transform: [{ scale: 0.25 + wave.value * 0.75 }],
    opacity: active ? 0.28 * (1 - wave.value) : 0,
  }));
  const ringsStyle = useAnimatedStyle(() => ({ transform: [{ scale: 1 + breath.value * 0.04 }] }));

  return (
    <View style={styles.box} pointerEvents="none">
      <Animated.View style={[styles.circle(OUTER), { backgroundColor: colour }, waveStyle]} />
      <Animated.View style={[StyleSheet.absoluteFill, styles.centre, ringsStyle]}>
        {RING_SIZES.map((size) => (
          <View
            key={size}
            style={[
              styles.circle(size),
              styles.ring,
              { borderColor: withAlpha(colour, active ? 0.55 : 0.35) },
            ]}
          />
        ))}
      </Animated.View>
      <View style={[styles.circle(DISC), { backgroundColor: withAlpha(colour, active ? 0.16 : 0.1) }]} />
      {children}
    </View>
  );
}

const styles = {
  ...StyleSheet.create({
    box: { width: OUTER, height: OUTER, alignItems: "center", justifyContent: "center" },
    centre: { alignItems: "center", justifyContent: "center" },
    ring: { borderWidth: 1.5, borderStyle: "dashed" },
  }),
  circle: (size: number) => ({
    position: "absolute" as const,
    width: size,
    height: size,
    borderRadius: size / 2,
  }),
};
