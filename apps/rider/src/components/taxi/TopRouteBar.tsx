/** TopRouteBar — floating "✕  Home → Destination  +" bar over the options map (design 3). */
import { Icon, Text, useTheme } from "@platform/ui";
import { Pressable, StyleSheet, View } from "react-native";

interface Props {
  from: string;
  to: string;
  top: number;
  onClose: () => void;
  onAdd: () => void;
}

export function TopRouteBar({ from, to, top, onClose, onAdd }: Props) {
  const { palette } = useTheme();
  return (
    <View style={[styles.bar, { top, backgroundColor: palette.background }]}>
      <Pressable accessibilityLabel="Close" onPress={onClose} hitSlop={10}>
        <Icon name="close" size={26} color={palette.text} />
      </Pressable>
      <View style={styles.middle}>
        <Text style={[styles.place, { color: palette.primary }]} numberOfLines={1}>
          {from}
        </Text>
        <Icon name="arrow-right" size={18} color={palette.text} />
        <Text style={styles.place} numberOfLines={1}>
          {to}
        </Text>
      </View>
      <Pressable accessibilityLabel="Edit route" onPress={onAdd} hitSlop={10}>
        <Icon name="plus" size={26} color={palette.text} />
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    position: "absolute",
    left: 12,
    right: 12,
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    borderRadius: 14,
    paddingHorizontal: 16,
    height: 60,
    shadowColor: "#000000",
    shadowOpacity: 0.1,
    shadowRadius: 10,
    shadowOffset: { width: 0, height: 3 },
    elevation: 4,
  },
  middle: { flex: 1, flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 6 },
  place: { fontSize: 16, fontWeight: "600", flexShrink: 1 },
});
