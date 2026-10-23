/** Snackbar — dark one-line notice over the map with an optional action ("Ride cancelled", "… Edit"). */
import { Text, useTheme } from "@platform/ui";
import { useEffect } from "react";
import { Pressable, StyleSheet } from "react-native";
import Animated, { FadeInDown, FadeOutDown } from "react-native-reanimated";

const AUTO_HIDE_MS = 6000;

interface Props {
  text: string;
  action?: string;
  onAction?: () => void;
  onHide: () => void;
  bottom: number;
}

export function Snackbar({ text, action, onAction, onHide, bottom }: Props) {
  const { palette } = useTheme();
  useEffect(() => {
    const id = setTimeout(onHide, AUTO_HIDE_MS);
    return () => clearTimeout(id);
  }, [onHide]);
  return (
    <Animated.View
      entering={FadeInDown.duration(280)}
      exiting={FadeOutDown}
      style={[styles.bar, { bottom, backgroundColor: palette.text }]}
    >
      <Text style={[styles.text, { color: palette.background }]} numberOfLines={2}>
        {text}
      </Text>
      {action ? (
        <Pressable onPress={onAction} hitSlop={8}>
          <Text style={[styles.action, { color: palette.highlight ?? palette.background }]}>{action}</Text>
        </Pressable>
      ) : null}
    </Animated.View>
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
    borderRadius: 10,
    paddingHorizontal: 16,
    paddingVertical: 14,
  },
  text: { flex: 1, fontSize: 14 },
  action: { fontSize: 15, fontWeight: "700" },
});
