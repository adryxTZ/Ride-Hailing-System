/**
 * FarePaymentCard — the end of a ride while the fare is owed: approve the
 * mobile-money prompt (with a countdown), or after a failure try again, use
 * another number, or pay cash; for cash rides, the amount to hand the driver.
 */
import { Icon, Text, TextField, useTheme, withAlpha } from "@platform/ui";
import { useState } from "react";
import { StyleSheet, View } from "react-native";
import Animated, { Easing, FadeInDown, LinearTransition } from "react-native-reanimated";
import { PillButton } from "@/components/common/PillButton";
import type { FareView } from "@/models/farePayment";
import { mmss } from "@/models/waiting";
import { SheetCard } from "./SheetCard";

const ENTER = FadeInDown.duration(260).easing(Easing.out(Easing.cubic));
const LAYOUT = LinearTransition.duration(240).easing(Easing.out(Easing.cubic));

interface Props {
  view: FareView;
  fare: string;
  busy: boolean;
  error?: string;
  onRetry: (phone?: string) => void;
  onCash: () => void;
  children?: React.ReactNode;
}

export function FarePaymentCard({ view, fare, busy, error, onRetry, onCash, children }: Props) {
  const { palette } = useTheme();
  const [other, setOther] = useState<string>();
  const icon =
    view.kind === "approve" ? "cellphone-message" : view.kind === "failed" ? "alert-circle-outline" : "cash";
  const tint = view.kind === "failed" ? (palette.danger ?? palette.primary) : palette.primary;
  return (
    <SheetCard style={styles.card}>
      <Animated.View key={view.kind} entering={ENTER} layout={LAYOUT} style={styles.body}>
        <View style={[styles.badge, { backgroundColor: withAlpha(tint, 0.12) }]}>
          <Icon name={icon} size={36} color={tint} />
        </View>
        <Text style={styles.title}>{titleOf(view, fare)}</Text>
        <Text variant="muted" style={styles.center}>
          {bodyOf(view)}
        </Text>
        {error ? <Text variant="error">{error}</Text> : null}
        {other !== undefined ? (
          <View style={styles.full}>
            <TextField
              label="Mobile money number"
              value={other}
              onChangeText={setOther}
              keyboardType="phone-pad"
              placeholder="07XX XXX XXX"
              autoFocus
            />
          </View>
        ) : null}
        <Actions
          view={view}
          busy={busy}
          other={other}
          setOther={setOther}
          onRetry={onRetry}
          onCash={onCash}
        />
      </Animated.View>
      {children}
    </SheetCard>
  );
}

function Actions({
  view,
  busy,
  other,
  setOther,
  onRetry,
  onCash,
}: Pick<Props, "view" | "busy" | "onRetry" | "onCash"> & { other?: string; setOther: (v?: string) => void }) {
  const { palette } = useTheme();
  if (other !== undefined) {
    return (
      <View style={styles.full}>
        <PillButton
          title="Send request"
          loading={busy}
          disabled={other.trim().length < 9}
          onPress={() => onRetry(other)}
        />
        <Text onPress={() => setOther(undefined)} style={[styles.link, { color: palette.primary }]}>
          Back
        </Text>
      </View>
    );
  }
  if (view.kind === "failed" && view.promptsLeft > 0) {
    return (
      <View style={styles.full}>
        <PillButton title="Try again" loading={busy} onPress={() => onRetry()} />
        <Text onPress={() => setOther("")} style={[styles.link, { color: palette.primary }]}>
          Use another number
        </Text>
        <Text onPress={onCash} style={[styles.link, { color: palette.primary }]}>
          Pay cash instead
        </Text>
      </View>
    );
  }
  if (view.kind === "failed") {
    return <PillButton title="Pay cash instead" loading={busy} onPress={onCash} style={styles.full} />;
  }
  if (view.kind === "cash" && view.canUseMobileMoney) {
    return (
      <Text onPress={() => onRetry()} style={[styles.link, { color: palette.primary }]}>
        Pay by mobile money instead
      </Text>
    );
  }
  return null;
}

function titleOf(view: FareView, fare: string): string {
  switch (view.kind) {
    case "approve":
      return "Approve on your phone";
    case "failed":
      return "Payment didn't go through";
    default:
      return `Pay ${fare}`;
  }
}

function bodyOf(view: FareView): string {
  switch (view.kind) {
    case "approve":
      return `We sent a payment request to ${view.phone}. Enter your PIN to pay. ${mmss(view.secondsLeft)} left.`;
    case "failed":
      return view.promptsLeft > 0
        ? `${view.reason} You can try again (${view.promptsLeft} left) or pay your driver in cash.`
        : `${view.reason} Please pay your driver in cash.`;
    case "cash":
      return "Please pay your driver in cash. Your trip ends once the driver confirms.";
    default:
      return "";
  }
}

const styles = StyleSheet.create({
  card: { paddingTop: 28 },
  body: { alignItems: "center", gap: 12 },
  badge: { width: 76, height: 76, borderRadius: 38, alignItems: "center", justifyContent: "center" },
  title: { fontSize: 24, fontWeight: "700", textAlign: "center" },
  center: { textAlign: "center" },
  full: { alignSelf: "stretch", gap: 4 },
  link: { fontSize: 16, fontWeight: "600", paddingVertical: 8, textAlign: "center" },
});
