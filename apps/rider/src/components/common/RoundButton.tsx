/** RoundButton — floating circular action over a map (back, profile, re-centre, schedule). */
import { Icon, ScalePressable, useTheme } from "@platform/ui";
import { type StyleProp, StyleSheet, type ViewStyle } from "react-native";

const SIZE = 52;

interface Props {
  icon: string;
  label: string;
  onPress: () => void;
  style?: StyleProp<ViewStyle>;
  /** "surface" = white floating button; "primary" = filled brand button. */
  tone?: "surface" | "primary";
  size?: number;
}

export function RoundButton({ icon, label, onPress, style, tone = "surface", size = SIZE }: Props) {
  const { palette } = useTheme();
  const primary = tone === "primary";
  return (
    <ScalePressable
      accessibilityLabel={label}
      onPress={onPress}
      style={[
        styles.btn,
        { width: size, height: size, borderRadius: size / 2 },
        { backgroundColor: primary ? palette.primary : palette.background },
        style,
      ]}
    >
      <Icon
        name={icon}
        size={24}
        color={primary ? (palette.on_primary ?? palette.background) : palette.text}
      />
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  btn: {
    alignItems: "center",
    justifyContent: "center",
    shadowColor: "#000000",
    shadowOpacity: 0.12,
    shadowRadius: 10,
    shadowOffset: { width: 0, height: 3 },
    elevation: 4,
  },
});
