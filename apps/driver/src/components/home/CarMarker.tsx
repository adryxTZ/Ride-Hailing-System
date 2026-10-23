/** CarMarker — the driver's own position: a brand-coloured disc with a car glyph (muted while offline). */
import { Icon, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";

const SIZE = 48;

export function CarMarker({ active }: { active: boolean }) {
  const { palette } = useTheme();
  const fill = active ? palette.primary : (palette.muted ?? palette.primary);
  return (
    <View style={[styles.disc, { backgroundColor: fill, borderColor: palette.background }]}>
      <Icon name="car" size={24} color={palette.on_primary ?? palette.background} />
    </View>
  );
}

const styles = StyleSheet.create({
  disc: {
    width: SIZE,
    height: SIZE,
    borderRadius: SIZE / 2,
    borderWidth: 3,
    alignItems: "center",
    justifyContent: "center",
  },
});
