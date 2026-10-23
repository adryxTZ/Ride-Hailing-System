/**
 * RouteScreen — design 2: "✕ Route", the pickup / stops / drop-off card, then
 * suggestions. While typing (3+ characters) the list shows matching saved and
 * recent places first, then place search results near the rider; otherwise
 * saved and recent places. After each choice the next empty field takes focus;
 * when everything is filled the ride options open.
 */
import { Icon, Text, useTheme } from "@platform/ui";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { ActivityIndicator, FlatList, Pressable, StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { PlaceRowSkeleton } from "@/components/common/Skeleton";
import { RecentRow } from "@/components/taxi/RecentRow";
import { type Field, RouteFields } from "@/components/taxi/RouteFields";
import { useBack } from "@/hooks/useBack";
import { CURRENT_LOCATION, useDefaultPickup } from "@/hooks/useDefaultPickup";
import { usePlaceSearch } from "@/hooks/usePlaceSearch";
import { usePlaceSuggestions } from "@/hooks/usePlaceSuggestions";
import { useService } from "@/hooks/useServices";
import { needsDetails } from "@/models/bookingDetails";
import { distanceKm } from "@/models/geo";
import { MAX_STOPS, type PlacePick, useBookingStore } from "@/services/bookingStore";

interface Row {
  key: string;
  icon: string;
  title: string;
  subtitle?: string;
  distanceKm?: number;
  pick: PlacePick;
}

export function RouteScreen() {
  const router = useRouter();
  const back = useBack();
  const insets = useSafeAreaInsets();
  const { palette } = useTheme();
  const booking = useBookingStore();
  const here = useDefaultPickup();
  const { service } = useService(booking.serviceKey ?? "");
  const [editing, setEditing] = useState<Field>("dropoff");
  const [query, setQuery] = useState("");
  const origin = booking.pickup?.point ?? here;
  const local = usePlaceSuggestions(query, origin);
  const remote = usePlaceSearch(query, origin);
  const rows = buildRows(
    local.items,
    remote.results,
    origin,
    editing === "pickup" && !remote.active ? here : undefined,
  );

  const edit = (f: Field) => {
    setEditing(f);
    setQuery("");
  };
  /** After a choice: the next empty field, or the ride options when all are set. */
  const advance = (
    stops = useBookingStore.getState().stops,
    dropoff = useBookingStore.getState().dropoff,
  ) => {
    const empty = stops.findIndex((s) => !s);
    if (empty >= 0) return edit(empty);
    if (!dropoff) return edit("dropoff");
    const st = useBookingStore.getState();
    router.push(needsDetails(service, st.pickup, here) ? "/book/details" : "/book/options");
  };
  const choose = (pick: PlacePick) => {
    if (editing === "pickup") booking.setPickup(pick);
    else if (editing === "dropoff") booking.setDropoff(pick);
    else booking.setStop(editing, pick);
    setQuery("");
    advance();
  };
  const addStop = () => {
    if (booking.stops.length >= MAX_STOPS) return;
    booking.addStop();
    edit(booking.stops.length); // the new, last slot
  };
  const removeStop = (i: number) => {
    booking.removeStop(i);
    if (editing === i || (typeof editing === "number" && editing > i)) edit("dropoff");
  };
  const swap = () => {
    if (!booking.pickup || !booking.dropoff) return;
    const { pickup, dropoff } = booking;
    booking.setPickup(dropoff);
    booking.setDropoff(pickup);
  };
  const field = typeof editing === "number" ? `stop-${editing}` : editing;
  const fromMap = useRef(false);
  const openMap = () => {
    fromMap.current = true;
    router.push({ pathname: "/book/pick", params: { field } });
  };
  // Back from "choose on the map": if that field is now set, continue like a list choice.
  useFocusEffect(
    useCallback(() => {
      if (!fromMap.current) return;
      fromMap.current = false;
      const st = useBookingStore.getState();
      const filled =
        editing === "pickup" ? !!st.pickup : editing === "dropoff" ? !!st.dropoff : !!st.stops[editing];
      if (filled) advance();
    }, [editing]),
  );

  return (
    <View style={[styles.root, { backgroundColor: palette.background, paddingTop: insets.top + 12 }]}>
      <View style={styles.header}>
        <Pressable accessibilityLabel="Close" onPress={() => back()} hitSlop={10}>
          <Icon name="close" size={28} color={palette.text} />
        </Pressable>
        <Text style={styles.title}>Route</Text>
        {remote.searching && remote.active ? <ActivityIndicator color={palette.primary} /> : null}
      </View>
      <RouteFields
        pickupLabel={booking.pickup?.address ?? CURRENT_LOCATION}
        dropoffLabel={booking.dropoff?.address}
        stopLabels={booking.stops.map((s) => s?.address)}
        canAddStop={booking.stops.length < MAX_STOPS}
        editing={editing}
        query={query}
        onQuery={setQuery}
        onEdit={edit}
        onMap={openMap}
        onSwap={swap}
        onAddStop={addStop}
        onRemoveStop={removeStop}
      />
      <FlatList
        data={rows}
        keyExtractor={(r) => r.key}
        keyboardShouldPersistTaps="handled"
        contentContainerStyle={{ paddingBottom: insets.bottom + 24 }}
        renderItem={({ item }) => (
          <RecentRow
            variant="plain"
            icon={item.icon}
            title={item.title}
            subtitle={item.subtitle}
            distanceKm={item.distanceKm}
            onPress={() => choose(item.pick)}
          />
        )}
        ListHeaderComponent={
          local.loading || (remote.active && remote.searching && rows.length === 0) ? <SkeletonList /> : null
        }
        ListEmptyComponent={
          remote.active && !remote.searching ? (
            <Text variant="muted" style={styles.empty}>
              No places found. Try another name or set it on the map.
            </Text>
          ) : null
        }
        ListFooterComponent={
          <RecentRow
            variant="plain"
            icon="map-marker-radius-outline"
            title="Set location on map"
            onPress={openMap}
          />
        }
      />
    </View>
  );
}

/** Saved/recent matches first, then search results (deduplicated by name), optionally "use my location". */
function buildRows(
  local: ReturnType<typeof usePlaceSuggestions>["items"],
  remote: ReturnType<typeof usePlaceSearch>["results"],
  origin: PlacePick["point"] | undefined,
  here: PlacePick["point"] | undefined,
): Row[] {
  const rows: Row[] = [];
  if (here)
    rows.push({
      key: "here",
      icon: "crosshairs-gps",
      title: "Use current location",
      pick: { point: here, address: CURRENT_LOCATION, source: "gps" },
    });
  const seen = new Set<string>();
  for (const s of local) {
    seen.add(s.title.toLowerCase());
    rows.push({
      key: s.key,
      icon: s.icon === "clock-outline" ? "clock-time-four-outline" : s.icon,
      title: s.title,
      subtitle: s.subtitle,
      distanceKm: s.distanceKm,
      pick: { point: s.point, address: s.title, source: s.key.startsWith("p-") ? "saved" : "search" },
    });
  }
  remote.forEach((p, i) => {
    if (seen.has(p.name.toLowerCase())) return;
    seen.add(p.name.toLowerCase());
    rows.push({
      key: `r-${i}-${p.name}`,
      icon: "map-marker-outline",
      title: p.name,
      subtitle: p.address,
      distanceKm: origin ? distanceKm(origin, p.location) : undefined,
      pick: { point: p.location, address: p.name, source: "search" },
    });
  });
  return rows;
}

const SKELETON_ROWS = 6;

/** Placeholder rows while places load. */
function SkeletonList() {
  return (
    <View>
      {Array.from({ length: SKELETON_ROWS }, (_, i) => (
        <PlaceRowSkeleton key={i} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, paddingHorizontal: 16, gap: 16 },
  header: { flexDirection: "row", alignItems: "center", gap: 24, paddingHorizontal: 4 },
  title: { fontSize: 20, fontWeight: "600", flex: 1 },
  empty: { paddingVertical: 16 },
});
