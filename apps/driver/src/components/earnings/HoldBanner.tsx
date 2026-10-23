/** HoldBanner — why the driver cannot go online, with the way out (pay, or contact support). */
import { Button, Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { Linking, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { type Hold, holdText, type Wallet } from "@/models/wallet";

interface Props {
  hold: Hold;
  wallet?: Wallet;
  brand?: string;
  /** The brand's support line; shown as a call button on staff holds. */
  supportPhone?: string;
  onPay?: () => void;
}

export function HoldBanner({ hold, wallet, brand, supportPhone, onPay }: Props) {
  const { palette } = useTheme();
  const tone = palette.danger ?? palette.primary;
  return (
    <Animated.View
      entering={FadeInDown.duration(280)}
      style={[
        styles.wrap,
        { backgroundColor: withAlpha(tone, 0.1), borderColor: tone, shadowColor: palette.text },
      ]}
    >
      <View style={styles.head}>
        <Icon name="pause-octagon-outline" size={22} color={tone} />
        <Text style={styles.title}>Your account is on hold</Text>
      </View>
      <Text variant="muted">{holdText(hold, wallet, brand)}</Text>
      {hold.kind === "DEBT" && onPay ? <Button title="Pay now" variant="danger" onPress={onPay} /> : null}
      {hold.kind === "MANUAL" && supportPhone ? (
        <Button
          title="Call support"
          variant="secondary"
          onPress={() => void Linking.openURL(`tel:${supportPhone}`)}
        />
      ) : null}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    borderRadius: 18,
    borderWidth: 1.5,
    padding: 16,
    gap: 10,
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  head: { flexDirection: "row", alignItems: "center", gap: 8 },
  title: { fontSize: 17, fontWeight: "700" },
});
