/**
 * TripPanel — the driver's assigned trip: what to do now, where, who, and one
 * main button for the next step (from the server's allowed transitions).
 */
import { Avatar, Button, Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { useRouter } from "expo-router";
import { Alert, Linking, Platform, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { useUnreadCount } from "@/hooks/useChat";
import { useNow } from "@/hooks/useNow";
import { driverFareState } from "@/models/farePayment";
import { canDriverCancel, nextAction, partyFor, stepFor } from "@/models/tripSteps";
import type { RoadRoute } from "@/services/routes";
import type { Trip, TripStatus } from "@/services/trips";
import { FareBlock } from "./FareBlock";
import { WaitingClock } from "./WaitingClock";

interface Props {
  trip: Trip;
  busy: boolean;
  error?: string;
  onStep: (to: TripStatus) => void;
  /** Ends the trip as a rider no-show (after the no-show wait). */
  onNoShow?: () => void;
  /** Road route to the current stop (distance and time). */
  road?: RoadRoute;
}

export function TripPanel({ trip, busy, error, onStep, onNoShow, road }: Props) {
  const { palette } = useTheme();
  const step = stepFor(trip);
  const stop = trip.stops.find((s) => s.kind === step.target);
  const waypoints = trip.stops.filter((s) => s.kind === "WAYPOINT");
  const showStops = step.target === "DROPOFF" && waypoints.length > 0;
  const fareState = driverFareState(trip, useNow(trip.payment?.status === "PENDING"));
  // No cash while the rider's mobile-money prompt is open: the money may still arrive.
  const action = fareState.kind === "waiting" ? undefined : nextAction(trip);
  const party = partyFor(trip, step.target);
  const router = useRouter();
  const unread = useUnreadCount(trip.id, "DRIVER");
  const fare = `${trip.currency} ${Math.round(Number(trip.final_fare ?? trip.estimated_fare)).toLocaleString("en-US")}`;
  const cancel = () =>
    Alert.alert("Cancel this trip?", "The rider will be matched with another driver.", [
      { text: "Keep trip", style: "cancel" },
      { text: "Cancel trip", style: "destructive", onPress: () => onStep("CANCELLED_BY_DRIVER") },
    ]);

  return (
    <Animated.View entering={FadeInDown.duration(280)} style={styles.wrap}>
      <View style={styles.head}>
        <Text style={styles.title}>{step.title}</Text>
        {road ? (
          <Text style={[styles.eta, { color: palette.primary }]}>
            {`${Math.max(1, Math.round(road.duration_s / 60))} min · ${(road.distance_m / 1000).toFixed(1)} km`}
          </Text>
        ) : null}
      </View>
      {trip.status === "ARRIVED_AT_PICKUP" ? <WaitingClock trip={trip} onNoShow={onNoShow} /> : null}
      {trip.notes ? (
        <View style={[styles.note, { backgroundColor: palette.surface }]}>
          <Icon name="note-text-outline" size={18} color={palette.text} />
          <Text style={styles.flex} numberOfLines={2}>
            {trip.notes}
          </Text>
        </View>
      ) : null}
      <View style={styles.address}>
        <Icon
          name={step.target === "PICKUP" ? "map-marker-account" : "flag-checkered"}
          size={22}
          color={palette.primary}
        />
        <Text style={styles.flex} numberOfLines={2}>
          {stop?.address ?? "Pinned location"}
        </Text>
        {stop ? (
          <ScalePressable
            accessibilityLabel="Navigate"
            onPress={() => navigate(stop.location, showStops ? waypoints.map((w) => w.location) : [])}
            style={[styles.round, { backgroundColor: palette.surface }]}
          >
            <Icon name="navigation-variant" size={22} color={palette.primary} />
          </ScalePressable>
        ) : null}
      </View>
      {showStops ? (
        <View style={styles.stops}>
          {waypoints.map((w, i) => (
            <View key={w.seq} style={styles.stopRow}>
              <View style={[styles.stopDot, { borderColor: palette.text }]} />
              <Text
                variant="muted"
                numberOfLines={1}
                style={styles.flex}
              >{`Stop ${i + 1}: ${w.address ?? "Pinned location"}`}</Text>
            </View>
          ))}
        </View>
      ) : null}
      <View style={[styles.rider, { borderColor: palette.border }]}>
        <Avatar name={party.name} size={44} />
        <View style={styles.flex}>
          <Text variant="muted">{party.role}</Text>
          <Text style={styles.name}>{party.name}</Text>
          <Text variant="muted">
            {[party.bookedBy, `${trip.payment_method === "CASH" ? "Cash" : trip.payment_method} · ${fare}`]
              .filter(Boolean)
              .join(" · ")}
          </Text>
        </View>
        <ScalePressable
          accessibilityLabel="Message rider"
          onPress={() => router.push({ pathname: "/chat/[id]", params: { id: trip.id } })}
          style={[styles.round, { backgroundColor: palette.surface }]}
        >
          <Icon name="message-processing" size={22} color={palette.text} />
          {unread > 0 ? (
            <View style={[styles.badge, { backgroundColor: palette.primary }]}>
              <Text style={[styles.badgeText, { color: palette.on_primary ?? palette.background }]}>
                {unread}
              </Text>
            </View>
          ) : null}
        </ScalePressable>
        {party.phone ? (
          <ScalePressable
            accessibilityLabel={`Call ${party.name}`}
            onPress={() => void Linking.openURL(`tel:${party.phone}`)}
            style={[styles.round, { backgroundColor: palette.surface }]}
          >
            <Icon name="phone" size={22} color={palette.text} />
          </ScalePressable>
        ) : null}
      </View>
      {error ? <Text variant="error">{error}</Text> : null}
      <FareBlock
        amount={fare}
        method={trip.payment_method}
        collect={trip.status === "AWAITING_PAYMENT"}
        state={fareState}
        earnings={trip.fees ? amountText(trip.fees.driver_earnings, trip.currency) : undefined}
        fee={trip.fees ? amountText(trip.fees.commission, trip.currency) : undefined}
      />
      {action ? (
        <Button
          title={action.to === "COMPLETED" ? `${action.label} · ${fare}` : action.label}
          onPress={() => onStep(action.to)}
          loading={busy}
        />
      ) : null}
      {canDriverCancel(trip) ? (
        <Text onPress={cancel} style={[styles.cancel, { color: palette.danger ?? palette.text }]}>
          Cancel trip
        </Text>
      ) : null}
    </Animated.View>
  );
}

type LatLng = { lat: number; lng: number };

const amountText = (v: string | number, currency: string) =>
  `${currency} ${Math.round(Number(v)).toLocaleString("en-US")}`;

/** Opens turn-by-turn navigation in the phone's maps app, through the stops when there are any. */
function navigate(p: LatLng, via: LatLng[] = []) {
  const stops = via.length ? `&waypoints=${via.map((w) => `${w.lat},${w.lng}`).join("|")}` : "";
  const web = `https://www.google.com/maps/dir/?api=1&travelmode=driving&destination=${p.lat},${p.lng}${stops}`;
  const url = Platform.OS === "android" && via.length === 0 ? `google.navigation:q=${p.lat},${p.lng}` : web;
  void Linking.openURL(url).catch(() => Linking.openURL(web));
}

const styles = StyleSheet.create({
  note: { flexDirection: "row", alignItems: "center", gap: 8, borderRadius: 12, padding: 10 },
  badge: {
    position: "absolute",
    top: -2,
    right: -2,
    minWidth: 18,
    height: 18,
    borderRadius: 9,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 4,
  },
  badgeText: { fontSize: 11, fontWeight: "700" },
  wrap: { gap: 14 },
  title: { fontSize: 22, fontWeight: "700", flexShrink: 1 },
  head: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 8 },
  eta: { fontSize: 15, fontWeight: "700" },
  address: { flexDirection: "row", alignItems: "center", gap: 10 },
  flex: { flex: 1 },
  round: { width: 46, height: 46, borderRadius: 23, alignItems: "center", justifyContent: "center" },
  rider: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingTop: 12,
  },
  name: { fontSize: 17, fontWeight: "600" },
  stops: { gap: 6, paddingLeft: 4 },
  stopRow: { flexDirection: "row", alignItems: "center", gap: 10 },
  stopDot: { width: 10, height: 10, borderRadius: 5, borderWidth: 2 },
  cancel: { textAlign: "center", fontSize: 15, fontWeight: "600", paddingVertical: 4 },
});
