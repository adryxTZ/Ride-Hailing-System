/** ProfileSwitch — personal / work segment with the "Work ride ⌄" label (design 3 footer). */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { Pressable, StyleSheet, View } from "react-native";

export type RideProfile = "personal" | "work";

interface Props {
  value: RideProfile;
  onChange: (p: RideProfile) => void;
  onOpen: () => void;
  label: string;
  /** Half sheet shows just a "Personal" chip; the full sheet shows the switch. */
  compact?: boolean;
}

export function ProfileSwitch({ value, onChange, onOpen, label, compact }: Props) {
  const { palette } = useTheme();
  if (compact) {
    return (
      <ScalePressable onPress={onOpen} style={[styles.chip, { borderColor: palette.border }]}>
        <Icon name={value === "work" ? "briefcase" : "account"} size={18} color={palette.text} />
        <Text style={styles.chipText}>{value === "work" ? "Work" : "Personal"}</Text>
      </ScalePressable>
    );
  }
  const seg = (p: RideProfile, icon: string) => (
    <Pressable
      accessibilityLabel={p === "work" ? "Work ride" : "Personal ride"}
      onPress={() => onChange(p)}
      style={[styles.seg, value === p ? { backgroundColor: palette.background } : null]}
    >
      <Icon name={icon} size={22} color={palette.text} />
    </Pressable>
  );
  return (
    <View style={styles.row}>
      <View style={[styles.track, { backgroundColor: palette.border ?? palette.surface }]}>
        {seg("personal", "account")}
        {seg("work", "briefcase")}
      </View>
      <ScalePressable onPress={onOpen} style={styles.label}>
        <Text style={styles.text}>{label}</Text>
        <Icon name="chevron-down" size={22} color={palette.text} />
      </ScalePressable>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 16 },
  track: { flexDirection: "row", borderRadius: 999, padding: 4 },
  seg: { width: 44, height: 44, borderRadius: 22, alignItems: "center", justifyContent: "center" },
  label: { flexDirection: "row", alignItems: "center", gap: 6 },
  text: { fontSize: 17 },
  chip: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    alignSelf: "flex-start",
    borderWidth: 1.5,
    borderRadius: 999,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  chipText: { fontSize: 15 },
});
