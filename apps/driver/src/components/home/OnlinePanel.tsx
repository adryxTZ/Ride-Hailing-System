/** OnlinePanel — the sheet while online: live search line, who is driving, and today's stats. */
import { Avatar, Banner, Text } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { SearchingLine } from "./SearchingLine";
import { type Stat, StatsStrip } from "./StatsStrip";

interface Props {
  name?: string;
  vehicleLabel: string;
  onlineFor: string;
  error?: string;
}

/** Until the driver-stats endpoint exists, only online time is real; the rest show a dash. */
function statsFor(onlineFor: string): Stat[] {
  return [
    { icon: "timer-outline", value: onlineFor, label: "Online" },
    { icon: "check-circle-outline", value: "—", label: "Accepted" },
    { icon: "star-outline", value: "—", label: "Rating" },
    { icon: "close-circle-outline", value: "—", label: "Cancelled" },
  ];
}

export function OnlinePanel({ name, vehicleLabel, onlineFor, error }: Props) {
  return (
    <View style={styles.wrap}>
      <Animated.View entering={FadeInDown.duration(280)}>
        <SearchingLine text="Finding trips near you" />
      </Animated.View>
      <Animated.View entering={FadeInDown.delay(60).duration(280)} style={styles.person}>
        <Avatar name={name} size={44} />
        <View style={styles.body}>
          <Text style={styles.name}>{name ?? "Driver"}</Text>
          <Text variant="muted">{vehicleLabel}</Text>
        </View>
      </Animated.View>
      <Animated.View entering={FadeInDown.delay(120).duration(280)}>
        <StatsStrip stats={statsFor(onlineFor)} />
      </Animated.View>
      {error ? <Banner icon="alert-circle-outline" title={error} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 14 },
  person: { flexDirection: "row", alignItems: "center", gap: 12 },
  body: { flex: 1, gap: 2 },
  name: { fontSize: 17, fontWeight: "700" },
});
