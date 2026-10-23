/** OfflinePanel — the sheet while offline (approved drivers): greeting, the big "Go online" action and the registered vehicle. */
import { Banner, Button, Text } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { VehicleCard } from "./VehicleCard";

interface Props {
  firstName?: string;
  vehicleLabel: string;
  /** Number plate of the registered vehicle. */
  plate?: string;
  busy: boolean;
  error?: string;
  onGoOnline: () => void;
}

export function OfflinePanel({ firstName, vehicleLabel, plate, busy, error, onGoOnline }: Props) {
  return (
    <View style={styles.wrap}>
      <Animated.View entering={FadeInDown.duration(280)}>
        <Text variant="muted">{firstName ? `Hi, ${firstName}` : "Hi there"}</Text>
        <Text style={styles.title}>You're offline</Text>
      </Animated.View>
      <Animated.View entering={FadeInDown.delay(60).duration(280)}>
        <Button title="Go online" onPress={onGoOnline} loading={busy} />
      </Animated.View>
      <Animated.View entering={FadeInDown.delay(120).duration(280)}>
        <VehicleCard label={vehicleLabel} plate={plate} />
      </Animated.View>
      {error ? <Banner icon="alert-circle-outline" title={error} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 14 },
  title: { fontSize: 24, fontWeight: "700" },
});
