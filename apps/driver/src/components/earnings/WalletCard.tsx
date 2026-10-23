/**
 * WalletCard — the driver's balance with the platform. After cash trips the
 * driver holds the platform's share, so the balance is negative ("You owe").
 * Shows when a hold starts, a warning before one, and the way to pay by mobile money.
 */
import { Button, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { money } from "@/models/earnings";
import { debtOf, ENTRY_LABEL, limitText, type Wallet, warningText } from "@/models/wallet";

const SHOWN_ENTRIES = 5;

export function WalletCard({ wallet, onPay }: { wallet: Wallet; onPay?: () => void }) {
  const { palette } = useTheme();
  const balance = Number(wallet.balance);
  const debt = debtOf(wallet);
  const owes = debt > 0;
  const tone = owes ? (palette.warning ?? palette.primary) : (palette.success ?? palette.primary);
  const title = owes ? "You owe" : balance > 0 ? "The platform owes you" : "Wallet balance";
  const warning = warningText(wallet);
  const min = Number(wallet.limits.min_payin);
  return (
    <View
      style={[
        styles.card,
        { backgroundColor: withAlpha(tone, 0.1), borderColor: tone, shadowColor: palette.text },
      ]}
    >
      <Text variant="muted">{title}</Text>
      <Text style={styles.big}>{money(owes ? debt : Math.abs(balance), wallet.currency)}</Text>
      <Text variant="muted">{owes ? limitText(wallet) : "Nothing to settle right now."}</Text>
      {warning ? <Text style={{ color: tone, fontWeight: "600" }}>{warning}</Text> : null}
      {owes && onPay && debt >= min ? <Button title="Pay now" onPress={onPay} /> : null}
      {owes && debt < min ? (
        <Text variant="muted">{`Payments start from ${money(min, wallet.currency)}; nothing to pay yet.`}</Text>
      ) : null}
      {wallet.entries.slice(0, SHOWN_ENTRIES).map((e) => {
        const credit = Number(e.amount) >= 0;
        return (
          <View key={`${e.reference}-${e.at}`} style={[styles.row, { borderTopColor: palette.border }]}>
            <Text variant="muted" style={styles.flex} numberOfLines={1}>
              {`${new Date(e.at).toLocaleDateString("en-GB", { day: "numeric", month: "short" })} · ${ENTRY_LABEL[e.kind] ?? e.kind}`}
            </Text>
            <Text
              style={[
                styles.amount,
                { color: credit ? (palette.success ?? palette.text) : (palette.danger ?? palette.text) },
              ]}
            >
              {`${credit ? "+" : "−"}${money(Math.abs(Number(e.amount)), wallet.currency)}`}
            </Text>
          </View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderRadius: 18,
    borderWidth: 1.5,
    padding: 16,
    gap: 8,
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  big: { fontSize: 28, fontWeight: "800" },
  row: { flexDirection: "row", gap: 8, borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 8 },
  flex: { flex: 1 },
  amount: { fontWeight: "600", fontVariant: ["tabular-nums"] },
});
