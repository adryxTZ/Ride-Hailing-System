/** Skeleton — grey placeholder blocks that gently pulse while content loads (route list, ride options). */
import { useTheme } from "@platform/ui";
import { useEffect } from "react";
import { type DimensionValue, StyleSheet, View } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withRepeat, withTiming } from "react-native-reanimated";

const PULSE_MS = 800;

function usePulse() {
  const t = useSharedValue(0);
  useEffect(() => {
    t.value = withRepeat(withTiming(1, { duration: PULSE_MS }), -1, true);
  }, [t]);
  return useAnimatedStyle(() => ({ opacity: 0.45 + t.value * 0.4 }));
}

export function Bone({
  width,
  height = 12,
  radius = 6,
}: {
  width: DimensionValue;
  height?: number;
  radius?: number;
}) {
  const { palette } = useTheme();
  const pulse = usePulse();
  return (
    <Animated.View
      style={[{ width, height, borderRadius: radius, backgroundColor: palette.border }, pulse]}
    />
  );
}

/** A place row placeholder: round icon, title and subtitle bars. */
export function PlaceRowSkeleton() {
  return (
    <View style={styles.row}>
      <Bone width={26} height={26} radius={13} />
      <View style={styles.texts}>
        <Bone width="70%" />
        <Bone width="45%" height={10} />
      </View>
    </View>
  );
}

/** A ride option placeholder: vehicle silhouette, name and price bars. */
export function OptionSkeleton() {
  return (
    <View style={styles.option}>
      <Bone width={76} height={44} radius={12} />
      <View style={styles.texts}>
        <Bone width="50%" height={14} />
        <Bone width="30%" height={10} />
      </View>
      <Bone width={60} height={14} />
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 16, paddingVertical: 18 },
  option: { flexDirection: "row", alignItems: "center", gap: 14, paddingVertical: 16, paddingHorizontal: 14 },
  texts: { flex: 1, gap: 8 },
});
