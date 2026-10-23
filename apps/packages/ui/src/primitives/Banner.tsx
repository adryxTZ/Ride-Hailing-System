import { Pressable, StyleSheet, View } from "react-native";
import { useTheme } from "../ThemeProvider";
import { Icon } from "./Icon";
import { Text } from "./Text";

/** Banner — tinted call-out with an icon, title and a highlighted line (e.g. "Finish setting up your profile"). */
export function Banner({
  icon,
  title,
  highlight,
  onPress,
}: {
  icon: string;
  title: string;
  highlight?: string;
  onPress?: () => void;
}) {
  const { palette } = useTheme();
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      style={[styles.wrap, { backgroundColor: palette.surface, borderColor: palette.border }]}
    >
      <View style={[styles.iconWrap, { backgroundColor: palette.primary }]}>
        <Icon name={icon} size={26} color={palette.on_primary ?? "#FFFFFF"} />
      </View>
      <View style={{ flex: 1, gap: 2 }}>
        <Text style={{ fontWeight: "600" }}>{title}</Text>
        {highlight ? <Text style={{ color: palette.accent, fontWeight: "600" }}>{highlight}</Text> : null}
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  wrap: {
    flexDirection: "row",
    alignItems: "center",
    gap: 14,
    padding: 14,
    borderRadius: 14,
    borderWidth: 1,
  },
  iconWrap: { width: 48, height: 48, borderRadius: 24, alignItems: "center", justifyContent: "center" },
});
