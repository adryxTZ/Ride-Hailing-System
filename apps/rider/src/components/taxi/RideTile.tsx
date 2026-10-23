/** RideTile — one of the 2×2 tiles on the taxi home ("Rides", "Schedule", …): picture, title, one line. */
import { ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { ServiceIcon } from "@/components/ServiceIcon";

interface Props {
  icon?: string;
  title: string;
  subtitle?: string;
  width: number;
  index: number;
  onPress: () => void;
}

export function RideTile({ icon, title, subtitle, width, index, onPress }: Props) {
  const { palette } = useTheme();
  return (
    <Animated.View entering={FadeInDown.delay(index * 60).duration(280)} style={{ width }}>
      <ScalePressable onPress={onPress} style={[styles.tile, { backgroundColor: palette.surface }]}>
        <ServiceIcon name={icon} size={64} />
        <Text style={styles.title} numberOfLines={1}>
          {title}
        </Text>
        {subtitle ? (
          <Text variant="muted" style={styles.subtitle} numberOfLines={1}>
            {subtitle}
          </Text>
        ) : null}
      </ScalePressable>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  tile: {
    borderRadius: 12,
    paddingVertical: 14,
    paddingHorizontal: 10,
    alignItems: "center",
    gap: 4,
    minHeight: 140,
  },
  title: { fontSize: 17, fontWeight: "500", marginTop: 4 },
  subtitle: { fontSize: 13 },
});
