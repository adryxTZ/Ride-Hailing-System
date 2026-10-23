import { Pressable, StyleSheet, View } from "react-native";
import { useTheme } from "../ThemeProvider";
import { Icon } from "./Icon";
import { Text } from "./Text";

/** Avatar — profile photo placeholder with initials or a camera prompt; optional edit badge. */
export function Avatar({
  name,
  size = 72,
  editable,
  onPress,
}: {
  name?: string;
  size?: number;
  editable?: boolean;
  onPress?: () => void;
}) {
  const { palette } = useTheme();
  const initials = (name ?? "")
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join("");
  return (
    <Pressable onPress={onPress} disabled={!onPress} style={{ width: size, height: size }}>
      <View
        style={[
          styles.circle,
          {
            width: size,
            height: size,
            borderRadius: size / 2,
            backgroundColor: palette.surface,
            borderColor: palette.border,
          },
        ]}
      >
        {initials ? (
          <Text style={{ fontSize: size / 2.6, fontWeight: "700", color: palette.primary }}>{initials}</Text>
        ) : (
          <Icon name="camera-plus" size={size / 2.4} />
        )}
      </View>
      {editable ? (
        <View style={[styles.badge, { backgroundColor: palette.accent }]}>
          <Icon name="pencil" size={14} color={palette.on_accent ?? "#FFFFFF"} />
        </View>
      ) : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  circle: { alignItems: "center", justifyContent: "center", borderWidth: 1 },
  badge: {
    position: "absolute",
    right: 0,
    top: 0,
    width: 26,
    height: 26,
    borderRadius: 13,
    alignItems: "center",
    justifyContent: "center",
  },
});
