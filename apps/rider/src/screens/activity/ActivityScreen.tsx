/**
 * ActivityScreen — trip history (design: General screens / history): the latest
 * trip on a card with its map and "Rebook", then every earlier trip. Tapping a
 * trip opens its details and receipt. The list refreshes when a trip ends.
 */
import { Header, ScalePressable, Screen, Text, useTheme } from "@platform/ui";
import { useRouter } from "expo-router";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { EmptyHistory } from "@/components/activity/EmptyHistory";
import { TripRow } from "@/components/activity/TripRow";
import { UpcomingCard } from "@/components/activity/UpcomingCard";
import { TaxiMap } from "@/components/map/TaxiMap";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useServices } from "@/hooks/useServices";
import { useCancelTrip, useTripHistory } from "@/hooks/useTrips";
import { useBookingStore } from "@/services/bookingStore";
import { localized } from "@/services/catalog";
import type { Trip } from "@/services/trips";

const LATEST_MAP_HEIGHT = 170;

export function ActivityScreen() {
  const router = useRouter();
  const { palette } = useTheme();
  const { data: trips, isLoading } = useTripHistory();
  const iconFor = useClassIcon();
  const classOf = useClassName();
  const rebook = useRebook();
  const cancel = useCancelTrip();
  const upcoming = (trips ?? [])
    .filter((t) => t.status === "SCHEDULED")
    .sort((a, b) => (a.scheduled_for ?? "").localeCompare(b.scheduled_for ?? ""));
  const [latest, ...rest] = (trips ?? []).filter((t) => t.status !== "SCHEDULED");
  const open = (t: Trip) => router.push({ pathname: "/trip/[id]", params: { id: t.id } });

  return (
    <Screen align="top">
      <Header title="Activity" large />
      {isLoading ? <ActivityIndicator color={palette.primary} /> : null}
      {upcoming.length > 0 ? <Text style={styles.section}>Upcoming</Text> : null}
      {upcoming.map((t, i) => (
        <Animated.View key={t.id} entering={FadeInDown.delay(i * 60).duration(260)}>
          <UpcomingCard
            trip={t}
            className={classOf(t)}
            cancelling={cancel.isPending && cancel.variables?.id === t.id}
            onCancel={() => cancel.mutate({ id: t.id, reason: "plans_changed" })}
          />
        </Animated.View>
      ))}
      {upcoming.length > 0 && latest ? <Text style={styles.section}>Past</Text> : null}
      {!isLoading && !latest && upcoming.length === 0 ? <EmptyHistory /> : null}
      {latest ? (
        <Animated.View
          entering={FadeInDown.duration(280)}
          style={[styles.latest, { borderColor: palette.border, backgroundColor: palette.background }]}
        >
          <View style={styles.map} pointerEvents="none">
            <TaxiMap
              centre={latest.stops[0]?.location ?? { lat: -6.79, lng: 39.21 }}
              insets={{ top: 10, bottom: 10 }}
              pickup={latest.stops.find((s) => s.kind === "PICKUP")?.location}
              dropoff={latest.stops.find((s) => s.kind === "DROPOFF")?.location}
              stops={latest.stops.filter((s) => s.kind === "WAYPOINT").map((s) => s.location)}
              showsUser={false}
            />
          </View>
          <View style={styles.latestBody}>
            <TripRow trip={latest} icon={iconFor(latest)} onPress={() => open(latest)} divider={false} />
            <ScalePressable
              onPress={() => rebook(latest)}
              style={[styles.rebook, { backgroundColor: palette.surface }]}
            >
              <Text style={styles.rebookText}>Rebook</Text>
            </ScalePressable>
          </View>
        </Animated.View>
      ) : null}
      {rest.map((t, i) => (
        <Animated.View key={t.id} entering={FadeInDown.delay(Math.min(i, 8) * 40).duration(240)}>
          <TripRow trip={t} icon={iconFor(t)} onPress={() => open(t)} />
        </Animated.View>
      ))}
    </Screen>
  );
}

/** The ride type's name (Kawaida, XL …) from the catalogue. */
function useClassName() {
  const { data: services } = useServices();
  const { data: config } = useAppConfig();
  return (t: Trip) => {
    const cls = services
      ?.find((s) => s.key === t.service)
      ?.vehicle_classes.find((c) => c.key === t.vehicle_class);
    return cls ? localized(cls.name, config?.brand.locale ?? "en") : t.vehicle_class;
  };
}

/** The picture of a trip's vehicle class from the catalogue. */
function useClassIcon() {
  const { data: services } = useServices();
  return (t: Trip) =>
    services?.find((s) => s.key === t.service)?.vehicle_classes.find((c) => c.key === t.vehicle_class)?.icon;
}

/** Same pickup, stops and drop-off again: straight to the ride options. */
function useRebook() {
  const router = useRouter();
  const booking = useBookingStore();
  return (t: Trip) => {
    const at = (kind: string) => t.stops.find((s) => s.kind === kind);
    const pickup = at("PICKUP");
    const dropoff = at("DROPOFF");
    if (!pickup || !dropoff) return;
    booking.start(t.service);
    booking.setPickup({ point: pickup.location, address: pickup.address ?? "Pickup", source: "search" });
    t.stops
      .filter((s) => s.kind === "WAYPOINT")
      .forEach((s, i) => {
        booking.addStop();
        booking.setStop(i, { point: s.location, address: s.address ?? `Stop ${i + 1}`, source: "search" });
      });
    booking.setDropoff({ point: dropoff.location, address: dropoff.address ?? "Drop-off", source: "search" });
    booking.setVehicleClass(t.vehicle_class);
    router.push("/book/options");
  };
}

const styles = StyleSheet.create({
  section: { fontSize: 18, fontWeight: "700" },
  latest: { borderWidth: 1, borderRadius: 18, overflow: "hidden" },
  map: { height: LATEST_MAP_HEIGHT },
  latestBody: { paddingHorizontal: 14, paddingBottom: 14 },
  rebook: {
    alignSelf: "flex-start",
    marginLeft: 86,
    borderRadius: 22,
    paddingHorizontal: 18,
    paddingVertical: 10,
  },
  rebookText: { fontSize: 15, fontWeight: "600" },
});
