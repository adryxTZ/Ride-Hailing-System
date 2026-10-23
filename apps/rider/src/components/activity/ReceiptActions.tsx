/** ReceiptActions — the receipt number with "Download PDF" in English or Swahili. */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";

interface Props {
  number?: string;
  busy: boolean;
  error?: string;
  onOpen: (lang: "en" | "sw") => void;
}

export function ReceiptActions({ number, busy, error, onOpen }: Props) {
  const { palette } = useTheme();
  if (!number) return null;
  return (
    <View style={styles.wrap}>
      <View style={styles.row}>
        <Icon name="receipt" size={20} color={palette.primary} />
        <Text style={styles.flex}>{`Receipt ${number}`}</Text>
        {busy ? <ActivityIndicator color={palette.primary} /> : null}
      </View>
      <View style={styles.row}>
        <ScalePressable
          accessibilityRole="button"
          onPress={() => onOpen("en")}
          disabled={busy}
          style={[styles.pill, { backgroundColor: palette.surface }]}
        >
          <Icon name="file-download-outline" size={18} color={palette.text} />
          <Text style={styles.pillText}>Download PDF</Text>
        </ScalePressable>
        <ScalePressable
          accessibilityRole="button"
          accessibilityLabel="Download the receipt in Swahili"
          onPress={() => onOpen("sw")}
          disabled={busy}
          style={[styles.pill, { backgroundColor: palette.surface }]}
        >
          <Text style={styles.pillText}>Kiswahili</Text>
        </ScalePressable>
      </View>
      {error ? <Text variant="error">{error}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 10, alignSelf: "stretch" },
  row: { flexDirection: "row", alignItems: "center", gap: 10 },
  flex: { flex: 1, fontWeight: "600" },
  pill: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    borderRadius: 999,
    paddingHorizontal: 14,
    paddingVertical: 10,
  },
  pillText: { fontWeight: "600" },
});
