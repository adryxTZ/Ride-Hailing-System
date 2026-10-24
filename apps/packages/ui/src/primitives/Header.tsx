import { Pressable, StyleSheet, View } from "react-native";
import { useTheme } from "../ThemeProvider";
import { Icon } from "./Icon";
import { Text } from "./Text";

interface Props {
  title?: string;
  /** "back" renders an arrow, "close" an X; omit for none. */
  leading?: "back" | "close";
  onLeading?: () => void;
  trailingIcon?: string;
  onTrailing?: () => void;
  /** Large title below the bar (design: "Payment", "Safety"). */
  large?: boolean;
}

/** Header — top bar with back/close, title, optional trailing action. */
export function Header({ title, leading, onLeading, trailingIcon, onTrailing, large }: Props) {
  const { palette } = useTheme();
  return (
    <View style={styles.wrap}>
      <View style={styles.bar}>
        {leading ? (
          <Pressable
            onPress={onLeading}
            hitSlop={12}
            style={styles.action}
            accessibilityRole="button"
            accessibilityLabel={leading === "back" ? "Back" : "Close"}
          >
            <Icon name={leading === "back" ? "arrow-left" : "close"} size={26} color={palette.text} />
          </Pressable>
        ) : (
          <View style={styles.action} />
        )}
        {!large && title ? <Text style={styles.barTitle}>{title}</Text> : <View style={{ flex: 1 }} />}
        {trailingIcon ? (
          <Pressable
            onPress={onTrailing}
            hitSlop={12}
            style={styles.action}
            accessibilityRole="button"
            accessibilityLabel={trailingIcon}
          >
            <Icon name={trailingIcon} size={24} color={palette.text} />
          </Pressable>
        ) : (
          <View style={styles.action} />
        )}
      </View>
      {large && title ? <Text variant="title">{title}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 12 },
  bar: { flexDirection: "row", alignItems: "center", minHeight: 44 },
  action: { width: 40, alignItems: "flex-start", justifyContent: "center" },
  barTitle: { flex: 1, fontSize: 18, fontWeight: "700" },
});
