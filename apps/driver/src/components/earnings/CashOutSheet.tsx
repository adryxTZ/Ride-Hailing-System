/**
 * CashOutSheet — how much to cash out (prefilled with everything available),
 * sent to the account phone only. Shows the server's answer when it refuses.
 */
import { Button, Icon, Text, TextField, useTheme, withAlpha } from "@platform/ui";
import { useState } from "react";
import { Modal, Pressable, StyleSheet } from "react-native";
import Animated, { Easing, FadeIn, FadeOut, SlideInDown, SlideOutDown } from "react-native-reanimated";
import { money } from "@/models/earnings";

const ENTER_MS = 280;
const EXIT_MS = 240;
const ease = Easing.out(Easing.cubic);

interface Props {
  bottom: number;
  available: number;
  min: number;
  currency: string;
  phone?: string;
  busy: boolean;
  error?: string;
  onSubmit: (amount: number) => void;
  onClose: () => void;
}

export function CashOutSheet({
  bottom,
  available,
  min,
  currency,
  phone,
  busy,
  error,
  onSubmit,
  onClose,
}: Props) {
  const { palette } = useTheme();
  const [amount, setAmount] = useState(String(available));
  const value = Number(amount) || 0;
  const valid = value >= min && value <= available;
  return (
    <Modal transparent visible animationType="none" onRequestClose={busy ? undefined : onClose}>
      <Animated.View
        entering={FadeIn.duration(ENTER_MS).easing(ease)}
        exiting={FadeOut.duration(EXIT_MS).easing(ease)}
        style={[StyleSheet.absoluteFill, styles.scrim, { backgroundColor: withAlpha(palette.text, 0.35) }]}
      >
        <Pressable
          style={StyleSheet.absoluteFill}
          onPress={busy ? undefined : onClose}
          accessibilityLabel="Close"
        />
        <Animated.View
          entering={SlideInDown.duration(ENTER_MS).easing(ease)}
          exiting={SlideOutDown.duration(EXIT_MS).easing(ease)}
          style={[styles.sheet, { backgroundColor: palette.background, paddingBottom: bottom + 20 }]}
        >
          <Pressable
            onPress={onClose}
            disabled={busy}
            hitSlop={8}
            style={[styles.close, { backgroundColor: palette.surface }]}
            accessibilityLabel="Close"
            accessibilityRole="button"
          >
            <Icon name="close" size={22} color={palette.text} />
          </Pressable>
          <Text style={styles.title}>Cash out</Text>
          <Text variant="muted">
            {`You can cash out up to ${money(available, currency)}. Minimum ${money(min, currency)}.`}
          </Text>
          <TextField
            label="Amount"
            value={amount}
            onChangeText={(t) => setAmount(t.replace(/\D/g, ""))}
            keyboardType="number-pad"
            error={
              amount && !valid
                ? `Enter an amount from ${money(min, currency)} to ${money(available, currency)}`
                : undefined
            }
          />
          <Text variant="muted">
            {phone ? `Sent to your account phone ${phone}.` : "Sent to your account phone."}
          </Text>
          {error ? <Text variant="error">{error}</Text> : null}
          <Button
            title={`Cash out ${money(value, currency)}`}
            onPress={() => valid && onSubmit(value)}
            disabled={!valid}
            loading={busy}
          />
        </Animated.View>
      </Animated.View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  scrim: { justifyContent: "flex-end" },
  sheet: { borderTopLeftRadius: 24, borderTopRightRadius: 24, padding: 20, gap: 14 },
  close: {
    position: "absolute",
    top: 12,
    right: 12,
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: "center",
    justifyContent: "center",
  },
  title: { fontSize: 22, fontWeight: "700", marginTop: 8 },
});
