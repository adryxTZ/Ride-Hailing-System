/** TopBar — floating header over the map: avatar (opens Account), status title, online switch (approved drivers). */
import { Avatar, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeIn, FadeOut } from "react-native-reanimated";
import { OnlineSwitch } from "./OnlineSwitch";

interface Props {
  name?: string;
  online: boolean;
  busy: boolean;
  top: number;
  onAvatar: () => void;
  onToggle: (next: boolean) => void;
  /** False until the driver is approved: the switch is hidden, not just disabled. */
  canGoOnline?: boolean;
}

export function TopBar({ name, online, busy, top, onAvatar, onToggle, canGoOnline = true }: Props) {
  const { palette } = useTheme();
  const dot = online ? (palette.success ?? palette.primary) : (palette.muted ?? palette.border);
  return (
    <View style={[styles.bar, { top }]}>
      <View style={[styles.chip, { backgroundColor: palette.background, borderColor: palette.border }]}>
        <Avatar name={name} size={36} onPress={onAvatar} />
      </View>
      <View style={[styles.title, { backgroundColor: palette.background, borderColor: palette.border }]}>
        <View style={[styles.dot, { backgroundColor: dot }]} />
        <Animated.View
          key={online ? "on" : "off"}
          entering={FadeIn.duration(220)}
          exiting={FadeOut.duration(120)}
        >
          <Text style={styles.label}>{online ? "Online" : "Offline"}</Text>
        </Animated.View>
      </View>
      {canGoOnline || online ? (
        <View
          style={[
            styles.chip,
            styles.switch,
            { backgroundColor: palette.background, borderColor: palette.border },
          ]}
        >
          <OnlineSwitch value={online} onChange={onToggle} disabled={busy} />
        </View>
      ) : (
        <View style={styles.placeholder} />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    position: "absolute",
    left: 16,
    right: 16,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  chip: { borderRadius: 999, borderWidth: 1, padding: 4 },
  switch: { paddingHorizontal: 6, paddingVertical: 6 },
  placeholder: { width: 44 },
  title: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    borderRadius: 999,
    borderWidth: 1,
    paddingHorizontal: 18,
    paddingVertical: 10,
  },
  dot: { width: 10, height: 10, borderRadius: 5 },
  label: { fontSize: 18, fontWeight: "700" },
});
