/** PaymentSummaryCard — "Payment method · Category: X · Cash  TZS 7,500 ⓘ" on an active trip. */
import { Icon, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { SheetCard } from "./SheetCard";

const METHOD_LABEL: Record<string, { label: string; icon: string }> = {
  CASH: { label: "Cash", icon: "cash" },
  CARD: { label: "Card", icon: "credit-card-outline" },
  MOBILE_MONEY: { label: "Mobile money", icon: "cellphone" },
  WALLET: { label: "Balance", icon: "wallet-outline" },
};

export function PaymentSummaryCard({
  category,
  method,
  amount,
}: {
  category?: string;
  method?: string;
  amount: string;
}) {
  const { palette } = useTheme();
  const m = METHOD_LABEL[method ?? "CASH"] ?? METHOD_LABEL.CASH;
  return (
    <SheetCard>
      <Text style={styles.title}>Payment method</Text>
      {category ? <Text variant="muted">Category: {category}</Text> : null}
      <View style={styles.row}>
        <Icon name={m.icon} size={24} color={palette.success ?? palette.primary} />
        <Text style={styles.flex}>{m.label}</Text>
        <Text style={styles.amount}>{amount}</Text>
        <Icon name="information-outline" size={20} color={palette.text} />
      </View>
    </SheetCard>
  );
}

const styles = StyleSheet.create({
  title: { fontSize: 20, fontWeight: "700" },
  row: { flexDirection: "row", alignItems: "center", gap: 12, marginTop: 6 },
  flex: { flex: 1, fontSize: 16 },
  amount: { fontSize: 16, fontWeight: "600" },
});
