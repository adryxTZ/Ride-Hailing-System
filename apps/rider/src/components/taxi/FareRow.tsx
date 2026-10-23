/** FareRow — the price of the ride and how it is paid, always visible on the trip card. */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";

const METHOD: Record<string, { label: string; icon: string }> = {
  CASH: { label: "Cash", icon: "cash" },
  CARD: { label: "Card", icon: "credit-card-outline" },
  MOBILE_MONEY: { label: "Mobile money", icon: "cellphone" },
  WALLET: { label: "Balance", icon: "wallet-outline" },
};

export function FareRow({ amount, method }: { amount: string; method?: string }) {
  const { palette } = useTheme();
  const m = METHOD[method ?? "CASH"] ?? METHOD.CASH;
  return (
    <View style={[styles.row, { backgroundColor: withAlpha(palette.primary, 0.08) }]}>
      <Icon name={m.icon} size={22} color={palette.success ?? palette.primary} />
      <Text style={styles.method}>{m.label}</Text>
      <Text style={styles.amount}>{amount}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    borderRadius: 12,
    paddingHorizontal: 14,
    paddingVertical: 12,
  },
  method: { flex: 1, fontSize: 15 },
  amount: { fontSize: 20, fontWeight: "800" },
});
