/** QueuedOfferTag — the pill on an offer made while the driver is finishing a trip. */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";

const TINT = 0.12;

export function QueuedOfferTag() {
  const { palette } = useTheme();
  return (
    <View style={[styles.pill, { backgroundColor: withAlpha(palette.primary, TINT) }]}>
      <Icon name="clock-fast" size={16} color={palette.primary} />
      <Text style={[styles.text, { color: palette.primary }]}>Next trip — after this one</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  pill: {
    alignSelf: "flex-start",
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 999,
  },
  text: { fontSize: 13, fontWeight: "600" },
});
