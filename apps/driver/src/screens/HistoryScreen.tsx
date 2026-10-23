/** HistoryScreen — the driver's trips, newest first, with what each one paid. */
import { Header, Screen, useTheme } from "@platform/ui";
import { ActivityIndicator } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { EmptyState } from "@/components/common/EmptyState";
import { DriverTripRow } from "@/components/history/DriverTripRow";
import { useTripHistory } from "@/hooks/useDriverTrips";

export function HistoryScreen() {
  const { palette } = useTheme();
  const { data: trips, isLoading } = useTripHistory();
  return (
    <Screen align="top">
      <Header title="History" large />
      {isLoading ? <ActivityIndicator color={palette.primary} /> : null}
      {!isLoading && !trips?.length ? (
        <EmptyState
          icon="clipboard-text-clock-outline"
          title="No trips yet"
          body="Go online from Home. Every trip you complete will be listed here."
        />
      ) : null}
      {trips?.map((t, i) => (
        <Animated.View key={t.id} entering={FadeInDown.delay(Math.min(i, 8) * 40).duration(240)}>
          <DriverTripRow trip={t} />
        </Animated.View>
      ))}
    </Screen>
  );
}
