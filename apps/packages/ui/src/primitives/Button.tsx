import { ActivityIndicator, Pressable, StyleSheet, Text } from "react-native";
import { useTheme } from "../ThemeProvider";

interface Props {
  title: string;
  onPress: () => void;
  loading?: boolean;
  disabled?: boolean;
  variant?: "primary" | "secondary" | "danger";
}

/** Filled button on the brand's primary, secondary or danger colour with the derived on-colour. */
export function Button({ title, onPress, loading, disabled, variant = "primary" }: Props) {
  const { palette } = useTheme();
  const background =
    variant === "primary"
      ? palette.primary
      : variant === "danger"
        ? (palette.danger ?? palette.primary)
        : palette.secondary;
  const foreground = palette.on_primary ?? "#FFFFFF";
  const inactive = disabled || loading;
  return (
    <Pressable
      onPress={onPress}
      disabled={inactive}
      style={({ pressed }) => [
        styles.button,
        { backgroundColor: background, opacity: inactive ? 0.6 : pressed ? 0.85 : 1 },
      ]}
    >
      {loading ? (
        <ActivityIndicator color={foreground} />
      ) : (
        <Text style={[styles.label, { color: foreground }]}>{title}</Text>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: { height: 52, borderRadius: 10, alignItems: "center", justifyContent: "center" },
  label: { fontSize: 16, fontWeight: "600" },
});
