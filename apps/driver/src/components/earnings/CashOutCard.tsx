/**
 * CashOutCard — what the platform owes the driver and the way to cash it out to
 * the account phone; while a cash-out is open, where it stands (and Cancel while
 * it waits for approval).
 */
import { Button, Icon, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { money } from "@/models/earnings";
import { type CashOutState, payoutText } from "@/models/payout";

interface Props {
  state: CashOutState;
  currency: string;
  busy: boolean;
  onCashOut: () => void;
  onCancel: (id: string) => void;
}

export function CashOutCard({ state, currency, busy, onCashOut, onCancel }: Props) {
  const { palette } = useTheme();
  if (state.kind === "off" || state.kind === "nothing") return null;
  return (
    <View style={[styles.card, { backgroundColor: palette.surface, shadowColor: palette.text }]}>
      <View style={styles.head}>
        <Icon name="bank-transfer-out" size={22} color={palette.primary} />
        <Text style={styles.title}>Cash out</Text>
      </View>
      {state.kind === "open" ? (
        <>
          <Text style={styles.big}>{money(Number(state.payout.amount), currency)}</Text>
          <Text variant="muted">{`${payoutText(state.payout)} · ${state.payout.phone}`}</Text>
          {state.payout.status === "REQUESTED" ? (
            <Button
              title="Cancel request"
              variant="secondary"
              loading={busy}
              onPress={() => onCancel(state.payout.id)}
            />
          ) : null}
        </>
      ) : (
        <>
          <Text style={styles.big}>{money(state.available, currency)}</Text>
          <Text variant="muted">
            {state.kind === "ready"
              ? "Earnings from mobile-money trips, ready to send to your phone."
              : `You can cash out from ${money(state.min, currency)}.`}
          </Text>
          {state.kind === "ready" ? <Button title="Cash out" onPress={onCashOut} /> : null}
        </>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderRadius: 18,
    padding: 16,
    gap: 10,
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  head: { flexDirection: "row", alignItems: "center", gap: 8 },
  title: { fontSize: 18, fontWeight: "700" },
  big: { fontSize: 28, fontWeight: "800" },
});
