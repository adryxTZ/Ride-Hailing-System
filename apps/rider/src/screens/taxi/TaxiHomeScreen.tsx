/**
 * TaxiHomeScreen — the app's home (design 1): live map with nearby cars on top; a sheet with
 * "Go wherever, whenever.", the 2×2 ride tiles, "Where to?" and recent places.
 */
import { Text, useTheme } from "@platform/ui";
import { useRouter } from "expo-router";
import { ScrollView, StyleSheet, useWindowDimensions, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { TaxiMap } from "@/components/map/TaxiMap";
import { RecentRow } from "@/components/taxi/RecentRow";
import { RideTile } from "@/components/taxi/RideTile";
import { WhereToBar } from "@/components/taxi/WhereToBar";
import { PARCEL_SERVICE_KEY, SERVICE_KEY } from "@/config";
import { useAppConfig } from "@/hooks/useAppConfig";
import { DEFAULT_CENTRE } from "@/hooks/useCurrentLocation";
import { useDefaultPickup } from "@/hooks/useDefaultPickup";
import { useNearbyVehicles } from "@/hooks/useNearbyVehicles";
import { usePlaceSuggestions } from "@/hooks/usePlaceSuggestions";
import { useService } from "@/hooks/useServices";
import { useBookingStore } from "@/services/bookingStore";
import { localized, type Service } from "@/services/catalog";

const MAP_SHARE = 0.3; // of the window height
const OVERLAP = 24; // sheet's rounded top overlaps the map
const PAD = 20;
const GAP = 12;
const RECENT_LIMIT = 5;

interface Tile {
  key: string;
  /** The service the tile books (rides or package delivery). */
  serviceKey: string;
  icon?: string;
  title: string;
  subtitle?: string;
  vehicleClass?: string;
}

export function TaxiHomeScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { height, width } = useWindowDimensions();
  const { palette } = useTheme();
  const { data: config } = useAppConfig();
  const booking = useBookingStore();
  const { service } = useService(SERVICE_KEY);
  const { service: parcel } = useService(PARCEL_SERVICE_KEY);
  const here = useDefaultPickup();
  const { data: cars } = useNearbyVehicles(here, SERVICE_KEY);
  const recents = usePlaceSuggestions("", here).items.slice(0, RECENT_LIMIT);
  const tileW = Math.floor((width - PAD * 2 - GAP) / 2);
  const later = () => router.push({ pathname: "/book/schedule", params: { from: "home" } });

  const pickTile = (t: Tile) => {
    if (!t.vehicleClass) return later();
    booking.start(t.serviceKey);
    booking.setVehicleClass(t.vehicleClass);
    router.push("/book/route");
  };
  const whereTo = () => {
    if (booking.serviceKey !== SERVICE_KEY) booking.start(SERVICE_KEY);
    router.push("/book/route");
  };

  return (
    <View style={[styles.root, { backgroundColor: palette.background }]}>
      <View style={{ height: Math.round(height * MAP_SHARE) + OVERLAP }}>
        <TaxiMap centre={here ?? DEFAULT_CENTRE} insets={{ top: insets.top, bottom: OVERLAP }} cars={cars} />
      </View>
      <ScrollView
        style={[styles.sheet, { backgroundColor: palette.background }]}
        contentContainerStyle={[styles.content, { paddingBottom: insets.bottom + 24 }]}
        showsVerticalScrollIndicator={false}
      >
        <View style={[styles.grabber, { backgroundColor: palette.border }]} />
        <Text style={styles.title}>{recents.length > 0 ? "Great to see you." : HEADLINE}</Text>
        <View style={styles.grid}>
          {tilesFor(service, parcel, config?.brand.locale ?? "en").map((t, i) => (
            <RideTile
              key={t.key}
              icon={t.icon}
              title={t.title}
              subtitle={t.subtitle}
              width={tileW}
              index={i}
              onPress={() => pickTile(t)}
            />
          ))}
        </View>
        <WhereToBar onPress={whereTo} onLater={later} />
        {recents.map((r) => (
          <RecentRow
            key={r.key}
            icon={r.icon === "clock-outline" ? "clock-time-four-outline" : r.icon}
            title={r.title}
            subtitle={r.subtitle}
            onPress={() => {
              booking.setDropoff({ point: r.point, address: r.title });
              router.push(booking.pickup ? "/book/options" : "/book/route");
            }}
          />
        ))}
      </ScrollView>
    </View>
  );
}

/** The greeting line under the map (design 1). */
const HEADLINE = "Go wherever, whenever.";

/** The first ride type, "Package" (boda delivery) when offered, then the next two ride types; XL and the rest are on the options screen. */
function tilesFor(service: Service | undefined, parcel: Service | undefined, locale: string): Tile[] {
  const classes: Tile[] = (service?.vehicle_classes ?? [])
    .filter((c) => c.status === "ACTIVE")
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
    .slice(0, 3)
    .map((c) => ({
      key: c.key,
      serviceKey: SERVICE_KEY,
      icon: c.icon,
      title: localized(c.name, locale),
      subtitle: localized(c.description, locale) || undefined,
      vehicleClass: c.key,
    }));
  const parcelClass = parcel?.vehicle_classes.find((c) => c.status === "ACTIVE");
  const send: Tile[] =
    parcel && parcelClass
      ? [
          {
            key: "parcel",
            serviceKey: PARCEL_SERVICE_KEY,
            icon: parcel.icon,
            title: localized(parcel.name, locale),
            subtitle: localized(parcel.description, locale) || undefined,
            vehicleClass: parcelClass.key,
          },
        ]
      : [];
  return [...classes.slice(0, 1), ...send, ...classes.slice(1, 3 - send.length + 1)].slice(0, 4);
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  sheet: { flex: 1, marginTop: -OVERLAP, borderTopLeftRadius: 24, borderTopRightRadius: 24 },
  content: { paddingHorizontal: PAD, paddingTop: 10, gap: 16 },
  grabber: { alignSelf: "center", width: 40, height: 5, borderRadius: 3 },
  title: { fontSize: 26, fontWeight: "700", marginTop: 4 },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: GAP },
});
