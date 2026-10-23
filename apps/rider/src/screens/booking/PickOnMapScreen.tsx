/** PickOnMapScreen — move the map under a fixed pin, name the spot, use it as pickup or drop-off. */
import { Icon, Text, TextField, useTheme } from "@platform/ui";
import { useLocalSearchParams } from "expo-router";
import { useEffect, useState } from "react";
import { type LayoutChangeEvent, StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { PillButton } from "@/components/common/PillButton";
import { RoundButton } from "@/components/common/RoundButton";
import { TaxiMap } from "@/components/map/TaxiMap";
import { useBack } from "@/hooks/useBack";
import { DEFAULT_CENTRE } from "@/hooks/useCurrentLocation";
import { useBookingStore } from "@/services/bookingStore";
import { reversePlace } from "@/services/places";
import type { Point } from "@/services/trips";

const PIN = 48;
const NAME_DELAY_MS = 600;

/** The street or landmark under the pin, looked up after the map stops moving. */
function useSpotName(at: Point): string | undefined {
  const [name, setName] = useState<string>();
  useEffect(() => {
    let cancelled = false;
    const id = setTimeout(() => {
      reversePlace(at).then((p) => !cancelled && setName(p?.name));
    }, NAME_DELAY_MS);
    return () => {
      cancelled = true;
      clearTimeout(id);
    };
  }, [at]);
  return name;
}
const MIN_NAME = 2;

export function PickOnMapScreen() {
  const back = useBack();
  const insets = useSafeAreaInsets();
  const { palette } = useTheme();
  const { field } = useLocalSearchParams<{ field?: string }>();
  const booking = useBookingStore();
  const isPickup = field === "pickup";
  const stopIndex = field?.startsWith("stop-") ? Number(field.slice(5)) : undefined;
  const current = isPickup ? booking.pickup : stopIndex != null ? booking.stops[stopIndex] : booking.dropoff;
  const start = current?.point ?? booking.pickup?.point ?? DEFAULT_CENTRE;
  const [initial] = useState<Point>(start);
  const [centre, setCentre] = useState<Point>(start);
  const [name, setName] = useState("");
  const [sheetH, setSheetH] = useState(260);
  const suggested = useSpotName(centre);

  const confirm = () => {
    const pick = { point: centre, address: name.trim() || suggested || "Pinned location" };
    if (isPickup) booking.setPickup(pick);
    else if (stopIndex != null) booking.setStop(stopIndex, pick);
    else booking.setDropoff(pick);
    back(); // the route screen moves on to the next empty field
  };

  return (
    <View style={styles.root}>
      <TaxiMap centre={initial} insets={{ top: insets.top, bottom: sheetH }} onCentreChange={setCentre} />
      <View pointerEvents="none" style={[styles.pinArea, { bottom: sheetH }]}>
        <View style={styles.pin}>
          <Icon name="map-marker" size={PIN} color={palette.primary} />
        </View>
      </View>
      <RoundButton
        icon="arrow-left"
        label="Back"
        onPress={() => back()}
        style={[styles.back, { top: insets.top + 8 }]}
      />
      <View
        onLayout={(e: LayoutChangeEvent) => setSheetH(e.nativeEvent.layout.height)}
        style={[styles.sheet, { backgroundColor: palette.background, paddingBottom: insets.bottom + 20 }]}
      >
        <Text style={styles.title}>
          {isPickup
            ? "Set pickup on map"
            : stopIndex != null
              ? `Set stop ${stopIndex + 1} on map`
              : "Set destination on map"}
        </Text>
        <Text variant="muted">Move the map until the pin is on the spot, then name it.</Text>
        <TextField
          placeholder={
            suggested ?? (isPickup ? "Pickup name, e.g. Gate 2" : "Destination name, e.g. Mlimani City")
          }
          value={name}
          onChangeText={setName}
        />
        <PillButton
          title={isPickup ? "Confirm pickup" : stopIndex != null ? "Confirm stop" : "Confirm destination"}
          disabled={name.trim().length > 0 && name.trim().length < MIN_NAME}
          onPress={confirm}
        />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  pinArea: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    alignItems: "center",
    justifyContent: "center",
  },
  pin: { marginBottom: PIN }, // the tip, not the centre, marks the spot
  back: { position: "absolute", left: 16 },
  sheet: {
    position: "absolute",
    left: 0,
    right: 0,
    bottom: 0,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    padding: 20,
    gap: 12,
    elevation: 8,
    shadowColor: "#000000",
    shadowOpacity: 0.1,
    shadowRadius: 14,
  },
  title: { fontSize: 22, fontWeight: "700" },
});
