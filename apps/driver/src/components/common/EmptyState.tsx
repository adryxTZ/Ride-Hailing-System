/** EmptyState — icon, title and one line of guidance for a list with nothing in it yet. */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";

export function EmptyState({ icon, title, body }: { icon: string; title: string; body: string }) {
  const { palette } = useTheme();
  return (
    <Animated.View entering={FadeInDown.duration(280)} style={styles.wrap}>
      <View style={[styles.badge, { backgroundColor: withAlpha(palette.primary, 0.1) }]}>
        <Icon name={icon} size={36} color={palette.primary} />
      </View>
      <Text style={styles.title}>{title}</Text>
      <Text variant="muted" style={styles.body}>
        {body}
      </Text>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  wrap: { alignItems: "center", gap: 10, paddingVertical: 48 },
  badge: { width: 80, height: 80, borderRadius: 40, alignItems: "center", justifyContent: "center" },
  title: { fontSize: 18, fontWeight: "700" },
  body: { textAlign: "center", paddingHorizontal: 24 },
});
