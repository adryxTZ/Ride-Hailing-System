/**
 * FareBreakdown — the receipt: fare lines (trip fare, surcharges, waiting…),
 * extra charges (toll, parking, levy … with their own labels), the booking fee and
 * the total, then the VAT and levy contained in it.
 * Taxes are included in the price, never added on top.
 */
import { Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { formatMoney } from "@/models/format";

export interface FareLineView {
  code: string;
  amount: string | number;
  /** The line's own wording (extra charges); used before the code table. */
  label?: { en?: string; sw?: string };
}

/** Booking fee, VAT and levy from the trip's fare split (all inside the total). */
export interface FeesView {
  booking_fee: string | number;
  vat: string | number;
  levy: string | number;
}

const LABEL: Record<string, string> = {
  BASE_FARE: "Trip fare",
  SURGE: "Busy-time surcharge",
  WAITING: "Waiting time",
  LOADERS: "Loaders",
  PICKUP_APPROACH: "Pickup distance",
  MINIMUM_FARE_ADJUSTMENT: "Minimum fare top-up",
  BOOKING_FEE: "Booking fee",
  FIXED_ROUTE: "Fixed route price",
  ROUNDING: "Rounding",
  PREVIOUS_CANCELLATION: "Earlier cancellation fee",
  CHARGE: "Extra charge",
};

/** Tax lines keep cents: they are reported to the tax authority and regulator exactly. */
const withCents = (v: string | number) =>
  Number(v).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

interface Props {
  lines: FareLineView[];
  total: string | number;
  currency: string;
  fees?: FeesView | null;
}

export function FareBreakdown({ lines, total, currency, fees }: Props) {
  const { palette } = useTheme();
  const shown = lines.filter((l) => Number(l.amount) !== 0 && !(fees && l.code === "BOOKING_FEE"));
  return (
    <View style={styles.wrap}>
      {shown.map((l, i) => (
        <Row
          key={`${l.code}-${i}`}
          label={l.label?.en ?? LABEL[l.code] ?? l.code}
          value={`${currency} ${formatMoney(l.amount)}`}
        />
      ))}
      {fees ? <Row label="Booking fee" value={`${currency} ${formatMoney(fees.booking_fee)}`} /> : null}
      <View style={[styles.row, styles.total, { borderTopColor: palette.border }]}>
        <Text style={styles.totalText}>Total</Text>
        <Text style={styles.totalText}>{`${currency} ${formatMoney(total)}`}</Text>
      </View>
      {fees ? (
        <>
          <Row label="Includes VAT" value={`${currency} ${withCents(fees.vat)}`} small />
          <Row label="Includes levy" value={`${currency} ${withCents(fees.levy)}`} small />
        </>
      ) : null}
    </View>
  );
}

function Row({ label, value, small }: { label: string; value: string; small?: boolean }) {
  return (
    <View style={styles.row}>
      <Text variant="muted" style={small ? styles.small : undefined}>
        {label}
      </Text>
      <Text variant="muted" style={small ? styles.small : undefined}>
        {value}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 8, alignSelf: "stretch" },
  row: { flexDirection: "row", justifyContent: "space-between" },
  total: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 10, marginTop: 2 },
  totalText: { fontSize: 17, fontWeight: "700" },
  small: { fontSize: 13 },
});
