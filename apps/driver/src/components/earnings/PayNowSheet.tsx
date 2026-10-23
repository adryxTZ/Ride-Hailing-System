/**
 * PayNowSheet — pay what you owe by mobile money. Amount prefilled to what lifts
 * the hold; the USSD prompt goes to the account phone unless another is typed.
 * While a prompt is waiting on the phone the sheet can be closed and reopened
 * without losing it.
 */
import { Button, Icon, Text, TextField, useTheme, withAlpha } from "@platform/ui";
import { useState } from "react";
import { Modal, Pressable, StyleSheet, View } from "react-native";
import Animated, { Easing, FadeIn, FadeOut, SlideInDown, SlideOutDown } from "react-native-reanimated";
import { money } from "@/models/earnings";
import { debtOf, type PayinPhase, suggestedPayment, type Wallet } from "@/models/wallet";
import { PayinStatus } from "./PayinStatus";

const ENTER_MS = 280;
const EXIT_MS = 240;
const ease = Easing.out(Easing.cubic);

interface Props {
  bottom: number;
  wallet: Wallet;
  phase: PayinPhase;
  error?: string;
  /** The account phone, used when no other number is typed. */
  phone?: string;
  /** Amount of the payment in progress (when adopted from the wallet). */
  pendingAmount?: number;
  successText: string;
  onPay: (amount: number, phone?: string) => void;
  onRetry: () => void;
  onClose: () => void;
}

export function PayNowSheet({
  bottom,
  wallet,
  phase,
  error,
  phone,
  pendingAmount,
  successText,
  onPay,
  onRetry,
  onClose,
}: Props) {
  const { palette } = useTheme();
  const [amount, setAmount] = useState(String(suggestedPayment(wallet)));
  const [number, setNumber] = useState("");
  const min = Number(wallet.limits.min_payin);
  const max = debtOf(wallet);
  const value = pendingAmount ?? (Number(amount.replace(/\D/g, "")) || 0);
  const valid = value >= min && value <= max;
  const busy = phase === "submitting";
  // A Modal sits above the floating tab bar, so the sheet is never drawn behind navigation.
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
          <Text style={styles.title}>Pay what you owe</Text>
          {phase === "idle" ? (
            <View style={styles.form}>
              <Text variant="muted">
                {`You owe ${money(max, wallet.currency)}. Minimum ${money(min, wallet.currency)}, whole shillings.`}
              </Text>
              <TextField
                label="Amount"
                value={amount}
                onChangeText={(t) => setAmount(t.replace(/\D/g, ""))}
                keyboardType="number-pad"
                error={
                  amount && !valid
                    ? `Enter an amount from ${money(min, wallet.currency)} to ${money(max, wallet.currency)}`
                    : undefined
                }
              />
              <TextField
                label="Mobile money number"
                value={number}
                onChangeText={setNumber}
                placeholder="07XX XXX XXX"
                keyboardType="phone-pad"
              />
              <Text variant="muted">
                {phone ? `Leave empty to use ${phone}.` : "The prompt goes to this number."}
              </Text>
              <Button
                title={`Pay ${money(value, wallet.currency)}`}
                onPress={() => valid && onPay(value, number.trim() || undefined)}
                disabled={!valid}
              />
            </View>
          ) : (
            <PayinStatus
              phase={phase}
              amountLabel={money(value, wallet.currency)}
              error={error}
              successText={successText}
            />
          )}
          {phase === "pending" ? (
            <Button title="Close, I'll approve on my phone" variant="secondary" onPress={onClose} />
          ) : null}
          {phase === "failed" ? <Button title="Try again" variant="secondary" onPress={onRetry} /> : null}
          {phase === "success" ? <Button title="Done" onPress={onClose} /> : null}
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
  form: { gap: 12 },
});
