/** AreaPicker — the regions of the driver's country, as selectable chips (where they will get rides). */
import { ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { Easing, LinearTransition } from "react-native-reanimated";

const LAYOUT = LinearTransition.duration(240).easing(Easing.out(Easing.cubic));

interface Props {
  areas: { id: string; name: string }[];
  value: string;
  onChange: (id: string) => void;
}

export function AreaPicker({ areas, value, onChange }: Props) {
  const { palette } = useTheme();
  return (
    <View style={styles.wrap}>
      <Text variant="muted">Region you will drive in</Text>
      <Animated.View layout={LAYOUT} style={styles.row}>
        {areas.map((a) => {
          const on = a.id === value;
          return (
            <ScalePressable
              key={a.id}
              accessibilityRole="radio"
              accessibilityState={{ selected: on }}
              accessibilityLabel={a.name}
              onPress={() => onChange(a.id)}
              style={[
                styles.chip,
                {
                  backgroundColor: on ? palette.primary : palette.surface,
                  borderColor: on ? palette.primary : palette.border,
                },
              ]}
            >
              <Text
                style={[
                  styles.text,
                  { color: on ? (palette.on_primary ?? palette.background) : palette.text },
                ]}
              >
                {a.name}
              </Text>
            </ScalePressable>
          );
        })}
      </Animated.View>
      <Text variant="muted">You get rides only in this region. Support can add more later.</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 6 },
  row: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  chip: {
    height: 44,
    borderRadius: 10,
    borderWidth: 1,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 14,
  },
  text: { fontWeight: "600", fontSize: 15 },
});
