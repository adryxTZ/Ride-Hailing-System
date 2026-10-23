/**
 * ChecklistRow — one step of the registration checklist: an icon, the step,
 * its state ("Add photo", "Uploaded", "Retake: blurry"), and a tick when done.
 */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";

export type RowTone = "todo" | "pending" | "ok" | "bad";

interface Props {
  icon: string;
  title: string;
  status: string;
  tone: RowTone;
  index: number;
  busy?: boolean;
  disabled?: boolean;
  onPress?: () => void;
}

const ENTER_STAGGER_MS = 60;
const TONE_ICON: Record<RowTone, string> = {
  todo: "chevron-right",
  pending: "check",
  ok: "check-circle",
  bad: "alert-circle",
};

export function ChecklistRow(p: Props) {
  const { palette } = useTheme();
  const toneColour = {
    todo: palette.muted ?? palette.text,
    pending: palette.primary,
    ok: palette.success ?? palette.primary,
    bad: palette.danger ?? palette.accent,
  }[p.tone];
  return (
    <Animated.View entering={FadeInDown.delay(p.index * ENTER_STAGGER_MS).duration(260)}>
      <ScalePressable
        accessibilityRole="button"
        accessibilityLabel={`${p.title}: ${p.status}`}
        accessibilityState={{ disabled: !!p.disabled, busy: !!p.busy }}
        onPress={p.onPress}
        disabled={p.disabled || p.busy || !p.onPress}
        style={[
          styles.row,
          { backgroundColor: palette.surface, borderColor: palette.border, opacity: p.disabled ? 0.5 : 1 },
        ]}
      >
        <View style={[styles.icon, { backgroundColor: withAlpha(palette.primary, 0.12) }]}>
          <Icon name={p.icon} size={24} color={palette.primary} />
        </View>
        <View style={styles.body}>
          <Text style={styles.title}>{p.title}</Text>
          <Text
            style={{ color: p.tone === "bad" ? toneColour : (palette.muted ?? palette.text) }}
            numberOfLines={2}
          >
            {p.status}
          </Text>
        </View>
        {p.busy ? (
          <ActivityIndicator color={palette.primary} />
        ) : (
          <Icon name={TONE_ICON[p.tone]} size={24} color={toneColour} />
        )}
      </ScalePressable>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    minHeight: 72,
    paddingHorizontal: 14,
    paddingVertical: 12,
    borderRadius: 18,
    borderWidth: 1,
    shadowColor: "#000000",
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  icon: { width: 48, height: 48, borderRadius: 14, alignItems: "center", justifyContent: "center" },
  body: { flex: 1, gap: 2 },
  title: { fontSize: 16, fontWeight: "700" },
});
