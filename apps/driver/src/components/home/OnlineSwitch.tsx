/** OnlineSwitch — the header toggle; the thumb slides across and the track fades to the brand colour. */
import { useTheme } from "@platform/ui";
import { useEffect } from "react";
import { Pressable, StyleSheet } from "react-native";
import Animated, {
  Easing,
  interpolateColor,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";

const TRACK_W = 56;
const TRACK_H = 32;
const PAD = 3;
const THUMB = TRACK_H - PAD * 2;
const TRAVEL = TRACK_W - THUMB - PAD * 2;

interface Props {
  value: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
}

export function OnlineSwitch({ value, onChange, disabled }: Props) {
  const { palette } = useTheme();
  const t = useSharedValue(value ? 1 : 0);
  useEffect(() => {
    t.value = withTiming(value ? 1 : 0, { duration: 220, easing: Easing.out(Easing.cubic) });
  }, [value, t]);
  const off = palette.border ?? palette.surface;
  const on = palette.primary;
  const track = useAnimatedStyle(() => ({ backgroundColor: interpolateColor(t.value, [0, 1], [off, on]) }));
  const thumb = useAnimatedStyle(() => ({ transform: [{ translateX: t.value * TRAVEL }] }));
  return (
    <Pressable
      accessibilityRole="switch"
      accessibilityLabel="Online"
      accessibilityState={{ checked: value, disabled }}
      onPress={() => onChange(!value)}
      disabled={disabled}
      hitSlop={10}
    >
      <Animated.View style={[styles.track, track, disabled ? styles.disabled : null]}>
        <Animated.View style={[styles.thumb, { backgroundColor: palette.background }, thumb]} />
      </Animated.View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  track: { width: TRACK_W, height: TRACK_H, borderRadius: TRACK_H / 2, padding: PAD },
  thumb: { width: THUMB, height: THUMB, borderRadius: THUMB / 2 },
  disabled: { opacity: 0.6 },
});
