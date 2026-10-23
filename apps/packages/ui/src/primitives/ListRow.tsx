import { Pressable, StyleSheet, View } from "react-native";
import { useTheme } from "../ThemeProvider";
import { Icon } from "./Icon";
import { Text } from "./Text";

interface Props {
  icon?: string;
  title: string;
  subtitle?: string;
  value?: string;
  onPress?: () => void;
  /** Hide the chevron (e.g. for rows with a trailing control). */
  chevron?: boolean;
  trailing?: React.ReactNode;
  destructive?: boolean;
}

/** ListRow — the standard menu/settings row: leading icon, title, optional subtitle/value, chevron. */
export function ListRow({
  icon,
  title,
  subtitle,
  value,
  onPress,
  chevron = true,
  trailing,
  destructive,
}: Props) {
  const { palette } = useTheme();
  const colour = destructive ? (palette.danger ?? palette.text) : palette.text;
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      style={({ pressed }) => [styles.row, { opacity: pressed ? 0.7 : 1 }]}
    >
      {icon ? <Icon name={icon} size={24} color={colour} /> : null}
      <View style={styles.body}>
        <Text style={[styles.title, { color: colour }]}>{title}</Text>
        {subtitle ? <Text variant="muted">{subtitle}</Text> : null}
      </View>
      {value ? <Text variant="muted">{value}</Text> : null}
      {trailing}
      {onPress && chevron && !trailing ? <Icon name="chevron-right" size={24} color={palette.muted} /> : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 14, paddingVertical: 14, minHeight: 56 },
  body: { flex: 1, gap: 2 },
  title: { fontSize: 16 },
});
