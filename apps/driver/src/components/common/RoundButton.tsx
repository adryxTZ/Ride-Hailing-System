/** RoundButton — small floating circular action (e.g. re-centre the map). */
import { Icon, ScalePressable, useTheme } from "@platform/ui";
import { type StyleProp, StyleSheet, type ViewStyle } from "react-native";

const SIZE = 46;

export function RoundButton({
  icon,
  onPress,
  style,
  label,
}: {
  icon: string;
  onPress: () => void;
  style?: StyleProp<ViewStyle>;
  label: string;
}) {
  const { palette } = useTheme();
  return (
    <ScalePressable
      accessibilityLabel={label}
      onPress={onPress}
      style={[styles.btn, { backgroundColor: palette.background, borderColor: palette.border }, style]}
    >
      <Icon name={icon} size={22} color={palette.text} />
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  btn: {
    position: "absolute",
    width: SIZE,
    height: SIZE,
    borderRadius: SIZE / 2,
    borderWidth: 1,
    alignItems: "center",
    justifyContent: "center",
  },
});
