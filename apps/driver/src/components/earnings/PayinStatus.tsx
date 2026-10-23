/** PayinStatus — the payment's progress inside the sheet: waiting for the PIN, received, or failed. */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";
import type { PayinPhase } from "@/models/wallet";

interface Props {
  phase: PayinPhase;
  amountLabel: string;
  error?: string;
  /** What to say once the money arrived (whether the driver can go online now). */
  successText: string;
}

export function PayinStatus({ phase, amountLabel, error, successText }: Props) {
  const { palette } = useTheme();
  const good = palette.success ?? palette.primary;
  const bad = palette.danger ?? palette.primary;
  if (phase === "submitting") {
    return (
      <Animated.View entering={FadeIn.duration(240)} style={styles.wrap}>
        <ActivityIndicator color={palette.primary} />
        <Text style={styles.title}>Sending the request</Text>
        <Text variant="muted" style={styles.body}>
          One moment.
        </Text>
      </Animated.View>
    );
  }
  if (phase === "pending") {
    return (
      <Animated.View entering={FadeIn.duration(240)} style={styles.wrap}>
        <ActivityIndicator color={palette.primary} />
        <Text style={styles.title}>Check your phone</Text>
        <Text variant="muted" style={styles.body}>{`Enter your PIN to approve ${amountLabel}.`}</Text>
      </Animated.View>
    );
  }
  if (phase === "success") {
    return (
      <Animated.View entering={FadeIn.duration(240)} style={styles.wrap}>
        <View style={[styles.badge, { backgroundColor: withAlpha(good, 0.14) }]}>
          <Icon name="check" size={36} color={good} />
        </View>
        <Text style={styles.title}>Payment received</Text>
        <Text variant="muted" style={styles.body}>
          {successText}
        </Text>
      </Animated.View>
    );
  }
  return (
    <Animated.View entering={FadeIn.duration(240)} style={styles.wrap}>
      <View style={[styles.badge, { backgroundColor: withAlpha(bad, 0.14) }]}>
        <Icon name="close" size={36} color={bad} />
      </View>
      <Text style={styles.title}>Payment not completed</Text>
      <Text variant="muted" style={styles.body}>
        {error ?? "Try again."}
      </Text>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  wrap: { alignItems: "center", gap: 8, paddingVertical: 8 },
  badge: { width: 72, height: 72, borderRadius: 36, alignItems: "center", justifyContent: "center" },
  title: { fontSize: 18, fontWeight: "700" },
  body: { textAlign: "center" },
});
