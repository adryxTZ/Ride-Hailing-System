/** PillButton — the large rounded primary action ("Confirm order", "Share"). */
import { ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, type StyleProp, StyleSheet, type ViewStyle } from "react-native";

interface Props {
  title: string;
  onPress: () => void;
  loading?: boolean;
  disabled?: boolean;
  style?: StyleProp<ViewStyle>;
}

export function PillButton({ title, onPress, loading, disabled, style }: Props) {
  const { palette } = useTheme();
  const ink = palette.on_primary ?? palette.background;
  return (
    <ScalePressable
      onPress={onPress}
      disabled={disabled || loading}
      style={[
        styles.btn,
        { backgroundColor: loading ? withAlpha(palette.primary, 0.35) : palette.primary },
        disabled ? styles.off : null,
        style,
      ]}
    >
      {loading ? (
        <ActivityIndicator color={palette.primary} />
      ) : (
        <Text style={[styles.text, { color: ink }]}>{title}</Text>
      )}
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  btn: {
    height: 62,
    borderRadius: 31,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 24,
  },
  off: { opacity: 0.5 },
  text: { fontSize: 18, fontWeight: "700" },
});
