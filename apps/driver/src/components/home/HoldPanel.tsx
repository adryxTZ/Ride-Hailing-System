/** HoldPanel — the home sheet while the driver is on hold: no "Go online", the reason and the way out. */
import { Text } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { HoldBanner } from "@/components/earnings/HoldBanner";
import { type Hold, holdTitle, type Wallet } from "@/models/wallet";
import { VehicleCard } from "./VehicleCard";

interface Props {
  firstName?: string;
  hold: Hold;
  wallet?: Wallet;
  brand?: string;
  supportPhone?: string;
  vehicleLabel: string;
  plate?: string;
  onPay: () => void;
}

export function HoldPanel({
  firstName,
  hold,
  wallet,
  brand,
  supportPhone,
  vehicleLabel,
  plate,
  onPay,
}: Props) {
  return (
    <View style={styles.wrap}>
      <Animated.View entering={FadeInDown.duration(280)}>
        <Text variant="muted">{firstName ? `Hi, ${firstName}` : "Hi there"}</Text>
        <Text style={styles.title}>{holdTitle(hold)}</Text>
      </Animated.View>
      <Animated.View entering={FadeInDown.delay(60).duration(280)}>
        <HoldBanner hold={hold} wallet={wallet} brand={brand} supportPhone={supportPhone} onPay={onPay} />
      </Animated.View>
      <Animated.View entering={FadeInDown.delay(120).duration(280)}>
        <VehicleCard label={vehicleLabel} plate={plate} />
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 14 },
  title: { fontSize: 24, fontWeight: "700" },
});
