/**
 * PrivacyRequestCard — one of my privacy requests: what it is, its status chip,
 * when it was asked and answered by, and the action that fits its status
 * (Download for a ready export, Cancel while nobody has picked it up).
 */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeInDown, LinearTransition } from "react-native-reanimated";
import { cardStyles, ENTER_MS, ENTER_STAGGER_MS } from "./cardStyle";
import {
  canCancel,
  canDownload,
  kindTitle,
  type PrivacyRequest,
  readyLine,
  requestedLine,
  type StatusTone,
  statusLabel,
  statusTone,
} from "./model";

interface Props {
  request: PrivacyRequest;
  index: number;
  /** Which action is running for this card, if any. */
  busy?: "download" | "cancel";
  onDownload: () => void;
  onCancel: () => void;
}

export function PrivacyRequestCard({ request, index, busy, onDownload, onCancel }: Props) {
  const { palette } = useTheme();
  const tones: Record<StatusTone, string> = {
    info: palette.info ?? palette.primary,
    ok: palette.success ?? palette.primary,
    bad: palette.danger ?? palette.primary,
    muted: palette.muted ?? palette.text,
  };
  const tone = tones[statusTone(request.status)];
  const ready = readyLine(request);
  return (
    <Animated.View
      entering={FadeInDown.delay(index * ENTER_STAGGER_MS).duration(ENTER_MS)}
      layout={LinearTransition.duration(ENTER_MS)}
      style={[cardStyles.card, { backgroundColor: palette.surface, shadowColor: palette.text }]}
    >
      <View style={styles.head}>
        <Text style={styles.title}>{kindTitle(request.kind)}</Text>
        <View style={[styles.chip, { backgroundColor: withAlpha(tone, 0.14) }]}>
          <Text style={[styles.chipText, { color: tone }]}>{statusLabel(request.status)}</Text>
        </View>
      </View>
      <Text variant="muted">{requestedLine(request)}</Text>
      {ready ? <Text style={{ color: tones.ok, fontWeight: "600" }}>{ready}</Text> : null}
      {request.status === "REJECTED" && request.reason ? <Text variant="muted">{request.reason}</Text> : null}
      {canDownload(request) ? (
        <CardAction icon="download" label="Download" busy={busy === "download"} onPress={onDownload} />
      ) : null}
      {canCancel(request) ? (
        <CardAction icon="close" label="Cancel request" busy={busy === "cancel"} onPress={onCancel} muted />
      ) : null}
    </Animated.View>
  );
}

/** A small tinted action button inside the card. */
function CardAction({
  icon,
  label,
  busy,
  muted,
  onPress,
}: {
  icon: string;
  label: string;
  busy: boolean;
  muted?: boolean;
  onPress: () => void;
}) {
  const { palette } = useTheme();
  const colour = muted ? (palette.muted ?? palette.text) : palette.primary;
  return (
    <ScalePressable
      accessibilityRole="button"
      accessibilityLabel={label}
      onPress={onPress}
      disabled={busy}
      style={[styles.action, { backgroundColor: withAlpha(colour, 0.12) }]}
    >
      {busy ? <ActivityIndicator color={colour} /> : <Icon name={icon} size={18} color={colour} />}
      <Text style={[styles.actionText, { color: colour }]}>{label}</Text>
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  head: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 8 },
  title: { fontSize: 16, fontWeight: "700", flexShrink: 1 },
  chip: { borderRadius: 999, paddingHorizontal: 10, paddingVertical: 3 },
  chipText: { fontSize: 13, fontWeight: "600" },
  action: {
    alignSelf: "flex-start",
    minHeight: 40,
    borderRadius: 12,
    paddingHorizontal: 12,
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  actionText: { fontWeight: "700" },
});
