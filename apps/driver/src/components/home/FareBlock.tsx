/**
 * FareBlock — what this trip pays and what the driver keeps, always visible. At
 * the payment step it becomes the call-out: "Collect TZS X in cash", or "The
 * rider is paying by mobile money" with a countdown while the prompt is open.
 */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import type { DriverFareState } from "@/models/farePayment";

const METHOD_LABEL: Record<string, string> = {
  CASH: "Cash",
  CARD: "Card",
  MOBILE_MONEY: "Mobile money",
  WALLET: "Balance",
};

interface Props {
  amount: string;
  method: string;
  collect: boolean;
  /** Where the fare stands at the payment step. */
  state?: DriverFareState;
  /** "TZS 5,400" — what the driver keeps after the platform fee. */
  earnings?: string;
  /** "TZS 600" — the platform fee on this fare. */
  fee?: string;
}

export function FareBlock({ amount, method, collect, state, earnings, fee }: Props) {
  const { palette } = useTheme();
  const accent = palette.success ?? palette.primary;
  const cash = method === "CASH";
  if (collect) {
    const waiting = state?.kind === "waiting";
    return (
      <View style={[styles.collect, { backgroundColor: withAlpha(accent, 0.12), borderColor: accent }]}>
        <Icon name={waiting ? "cellphone-message" : "cash-multiple"} size={30} color={accent} />
        <View style={styles.flex}>
          <Text variant="muted">
            {waiting ? "The rider is paying by mobile money" : "Collect from the rider"}
          </Text>
          <Text style={styles.big}>{amount}</Text>
          <Text variant="muted">
            {state?.kind === "waiting"
              ? `Waiting for their approval · ${Math.floor(state.secondsLeft / 60)}:${String(state.secondsLeft % 60).padStart(2, "0")}`
              : state?.kind === "collect"
                ? state.note
                : "in cash, then confirm below"}
          </Text>
          {earnings ? (
            <Text style={styles.keep}>{`You keep ${earnings}${fee ? ` · platform fee ${fee}` : ""}`}</Text>
          ) : null}
        </View>
      </View>
    );
  }
  return (
    <View style={[styles.row, { backgroundColor: palette.surface }]}>
      <Icon name={cash ? "cash" : "cellphone"} size={22} color={accent} />
      <Text style={styles.flex}>{`Trip fare · ${METHOD_LABEL[method] ?? method}`}</Text>
      <View style={styles.right}>
        <Text style={styles.amount}>{amount}</Text>
        {earnings ? <Text variant="muted">{`You earn ${earnings}`}</Text> : null}
      </View>
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
  amount: { fontSize: 20, fontWeight: "800" },
  right: { alignItems: "flex-end" },
  flex: { flex: 1 },
  collect: {
    flexDirection: "row",
    alignItems: "center",
    gap: 14,
    borderRadius: 14,
    borderWidth: 2,
    padding: 16,
  },
  big: { fontSize: 30, fontWeight: "800" },
  keep: { fontSize: 14, fontWeight: "600", marginTop: 4 },
});
