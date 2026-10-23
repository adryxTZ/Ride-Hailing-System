import { Children, type PropsWithChildren } from "react";
import { StyleSheet, View } from "react-native";
import { useTheme } from "../ThemeProvider";
import { Text } from "./Text";

/** Section — a card grouping rows with hairline separators and an optional title. */
export function Section({ title, children }: PropsWithChildren<{ title?: string }>) {
  const { palette } = useTheme();
  const items = Children.toArray(children);
  return (
    <View style={styles.wrap}>
      {title ? <Text style={styles.title}>{title}</Text> : null}
      <View style={[styles.card, { backgroundColor: palette.surface, borderColor: palette.border }]}>
        {items.map((child, i) => (
          <View key={`${i}-${String(child)}`}>
            {child}
            {i < items.length - 1 ? (
              <View style={[styles.separator, { backgroundColor: palette.border }]} />
            ) : null}
          </View>
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 8 },
  title: { fontSize: 18, fontWeight: "700" },
  card: {
    borderRadius: 18,
    borderWidth: 1,
    paddingHorizontal: 16,
    shadowColor: "#000000",
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  separator: { height: StyleSheet.hairlineWidth },
});
