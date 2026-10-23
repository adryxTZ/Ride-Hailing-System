/** EmptyHistory — Activity before the first trip. */
import { Icon, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";

export function EmptyHistory() {
  const { palette } = useTheme();
  return (
    <View style={[styles.empty, { backgroundColor: palette.surface, borderColor: palette.border }]}>
      <Icon name="history" size={40} color={palette.muted} />
      <Text variant="muted">Your trips will appear here.</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  empty: { alignItems: "center", gap: 10, padding: 32, borderRadius: 14, borderWidth: 1 },
});
