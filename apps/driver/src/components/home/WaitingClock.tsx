/** WaitingClock — at the pickup: how long the driver has waited, the free time left, the fee building up, and "No-show" once the wait allows it. */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { Alert, StyleSheet, View } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";
import { useNow } from "@/hooks/useNow";
import { mmss, noShowAfterS, waitingState, waitingTerms } from "@/models/waiting";
import type { Trip } from "@/services/trips";

export function WaitingClock({ trip, onNoShow }: { trip: Trip; onNoShow?: () => void }) {
  const { palette } = useTheme();
  const now = useNow(true);
  const w = waitingState(trip, now);
  if (!w) return null;
  const t = waitingTerms(trip);
  const noShowIn = Math.max(0, noShowAfterS(trip) - w.waitedS);
  const confirmNoShow = () =>
    Alert.alert(
      "Rider didn't show up?",
      "The trip ends and the rider pays the no-show fee. Part of it goes to you.",
      [
        { text: "Keep waiting", style: "cancel" },
        { text: "End trip", style: "destructive", onPress: onNoShow },
      ],
    );
  const tint = w.charging ? (palette.warning ?? palette.accent) : palette.primary;
  return (
    <Animated.View
      entering={FadeIn.duration(240)}
      style={[styles.box, { backgroundColor: withAlpha(tint, 0.1), borderColor: withAlpha(tint, 0.35) }]}
      accessibilityLabel={`Waiting ${mmss(w.waitedS)}`}
    >
      <Icon name="timer-sand" size={22} color={tint} />
      <View style={styles.flex}>
        <Text style={styles.big}>{`Waiting ${mmss(w.waitedS)}`}</Text>
        <Text variant="muted">
          {w.charging
            ? `Waiting fee ${trip.currency} ${w.fee.toLocaleString("en-US")} · ${t.ratePerMin}/min`
            : `Free for ${mmss(w.freeLeftS)} more, then ${trip.currency} ${t.ratePerMin}/min`}
        </Text>
      </View>
      {onNoShow && noShowAfterS(trip) > 0 && noShowIn === 0 ? (
        <ScalePressable
          accessibilityLabel="Rider didn't show up"
          onPress={confirmNoShow}
          style={[styles.noShow, { borderColor: tint }]}
        >
          <Text style={styles.noShowText}>No-show</Text>
        </ScalePressable>
      ) : null}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  box: { flexDirection: "row", alignItems: "center", gap: 12, borderWidth: 1, borderRadius: 14, padding: 12 },
  flex: { flex: 1, gap: 2 },
  big: { fontSize: 17, fontWeight: "700" },
  noShow: {
    minHeight: 44,
    paddingHorizontal: 14,
    borderRadius: 22,
    borderWidth: 1,
    justifyContent: "center",
  },
  noShowText: { fontWeight: "700" },
});
