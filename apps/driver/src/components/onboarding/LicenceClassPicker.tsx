/** LicenceClassPicker — the licence classes the driver's country lists, as selectable chips. */
import { ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { Easing, LinearTransition } from "react-native-reanimated";

const LAYOUT = LinearTransition.duration(240).easing(Easing.out(Easing.cubic));

interface Props {
  classes: string[];
  value: string;
  onChange: (cls: string) => void;
}

export function LicenceClassPicker({ classes, value, onChange }: Props) {
  const { palette } = useTheme();
  return (
    <View style={styles.wrap}>
      <Text variant="muted">Licence class</Text>
      <Animated.View layout={LAYOUT} style={styles.row}>
        {classes.map((c) => {
          const on = c === value;
          return (
            <ScalePressable
              key={c}
              accessibilityRole="radio"
              accessibilityState={{ selected: on }}
              accessibilityLabel={`Class ${c}`}
              onPress={() => onChange(c)}
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
                {c}
              </Text>
            </ScalePressable>
          );
        })}
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 6 },
  row: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  chip: {
    minWidth: 48,
    height: 44,
    borderRadius: 10,
    borderWidth: 1,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 12,
  },
  text: { fontWeight: "700", fontSize: 16 },
});
