/**
 * TripScreen — the rider's live trip, from "Looking for a driver" to the end
 * (taxi story 4 + 6 and the recording). The map, the sheet and the actions follow
 * the trip phase; updates arrive over the socket (polling is only a fallback).
 */
import { useTheme } from "@platform/ui";
import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import {
  Alert,
  type LayoutChangeEvent,
  ScrollView,
  Share,
  StyleSheet,
  useWindowDimensions,
} from "react-native";
import Animated, { FadeIn, SlideInDown } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { RoundButton } from "@/components/common/RoundButton";
import { Snackbar } from "@/components/common/Snackbar";
import { PulseRing } from "@/components/map/PulseRing";
import { TaxiMap } from "@/components/map/TaxiMap";
import { SafetySheet } from "@/components/safety/SafetySheet";
import { CancelConfirmSheet } from "@/components/taxi/CancelConfirmSheet";
import { MoreActions } from "@/components/taxi/MoreActions";
import { RouteSummaryCard } from "@/components/taxi/RouteSummaryCard";
import { SafetyPill } from "@/components/taxi/SafetyPill";
import { useRoute } from "@/hooks/useRoute";
import { useTripSafety } from "@/hooks/useTripSafety";
import { useTrip } from "@/hooks/useTrips";
import { formatMoney } from "@/models/format";
import { decodePolyline } from "@/models/geo";
import { useBookingStore } from "@/services/bookingStore";
import type { Trip } from "@/services/trips";
import { TripPayment, TripStatusBlock } from "./TripBlocks";
import { canCancel, phaseOf, type TripPhase } from "./tripPhase";

const LOOKING_MS = 2500;
const STREET_ZOOM = 16;
const WIDE_ZOOM = 12;
const SHEET_SHARE = 0.55;

export function TripScreen({ tripId }: { tripId?: string }) {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { height } = useWindowDimensions();
  const { palette } = useTheme();
  const { data: trip } = useTrip(tripId);
  const phase = phaseOf(trip);
  const [looking, setLooking] = useState(true);
  const [confirming, setConfirming] = useState(false);
  const [sheetH, setSheetH] = useState(height * SHEET_SHARE);
  const notice = useDriverCancelledNotice(phase);
  useEffect(() => {
    const id = setTimeout(() => setLooking(false), LOOKING_MS);
    return () => clearTimeout(id);
  }, []);
  useRebookWhenCancelled(phase);

  const pickup = trip?.stops.find((s) => s.kind === "PICKUP");
  const dropoff = trip?.stops.find((s) => s.kind === "DROPOFF");
  const waypoints = trip?.stops.filter((s) => s.kind === "WAYPOINT") ?? [];
  const pendingStops = waypoints.filter((s) => !s.arrived_at).map((s) => s.location);
  const driverAt = trip?.driver_location?.location;
  const toPickup = phase === "assigned" || phase === "arrived";
  const target = toPickup ? pickup?.location : dropoff?.location;
  const searching = phase === "searching";
  const road = useRoute(searching ? undefined : driverAt, target, toPickup ? [] : pendingStops).data;
  const line = road?.polyline
    ? decodePolyline(road.polyline)
    : driverAt && target
      ? [driverAt, target]
      : undefined;
  const safety = useTripSafety(trip);
  const share = safety.share;

  return (
    <Animated.View entering={FadeIn} style={styles.root}>
      <TaxiMap
        centre={pickup?.location ?? { lat: -6.79, lng: 39.21 }}
        insets={{ top: insets.top + 60, bottom: sheetH }}
        pickup={searching && looking ? undefined : toPickup || searching ? pickup?.location : undefined}
        pickupLabel="Pickup here"
        dropoff={toPickup || searching ? undefined : dropoff?.location}
        stops={toPickup || searching ? undefined : waypoints.map((s) => s.location)}
        driver={driverAt}
        route={!searching ? line : undefined}
        fitTo={driverAt && target && !searching ? [driverAt, target] : undefined}
        zoom={searching && !looking ? STREET_ZOOM : undefined}
        initialZoom={WIDE_ZOOM}
        showsUser={phase !== "riding"}
      />
      {searching && !looking ? (
        <Animated.View
          entering={FadeIn.delay(600)}
          pointerEvents="none"
          style={[styles.pulse, { bottom: sheetH }]}
        >
          <PulseRing />
        </Animated.View>
      ) : null}
      <RoundButton
        icon="account-outline"
        label="Account"
        onPress={() => router.navigate("/account")}
        style={[styles.profile, { top: insets.top + 8 }]}
      />
      {phase === "assigned" || phase === "arrived" || phase === "riding" ? (
        <SafetyPill onPress={() => safety.setOpen(true)} style={{ left: 16, bottom: sheetH + 16 }} />
      ) : null}
      {notice ? <Snackbar text={notice.text} bottom={sheetH + 12} onHide={notice.hide} /> : null}
      <Animated.View
        entering={SlideInDown.duration(320)}
        onLayout={(e: LayoutChangeEvent) => setSheetH(e.nativeEvent.layout.height)}
        style={[styles.sheet, { maxHeight: height * SHEET_SHARE, backgroundColor: palette.surface }]}
      >
        <ScrollView
          contentContainerStyle={[styles.content, { paddingBottom: insets.bottom + 16 }]}
          showsVerticalScrollIndicator={false}
        >
          <TripStatusBlock
            trip={trip}
            phase={phase}
            looking={looking}
            etaS={road?.duration_s}
            onCancel={() => setConfirming(true)}
          />
          {phase === "done" || phase === "no_driver" || phase === "no_show" ? null : (
            <>
              <RouteSummaryCard
                pickup={pickup?.address ?? undefined}
                stops={waypoints.map((s) => s.address ?? "Stop")}
                dropoff={dropoff?.address ?? undefined}
              />
              <TripPayment trip={trip} />
              <MoreActions actions={moreActions(phase, trip, share, () => setConfirming(true))} />
            </>
          )}
        </ScrollView>
      </Animated.View>
      {safety.open ? (
        <SafetySheet
          bottom={insets.bottom}
          emergencyPhone={safety.emergencyPhone}
          hasContacts={safety.hasContacts}
          sharing={safety.sharing}
          alerting={safety.alerting}
          status={safety.status}
          onShare={safety.share}
          onSos={safety.sos}
          onAddContacts={() => {
            safety.setOpen(false);
            router.push("/account/safety");
          }}
          onClose={() => safety.setOpen(false)}
        />
      ) : null}
      {confirming ? (
        <CancelConfirmSheet
          bottom={insets.bottom}
          fee={
            Number(trip?.cancel_fee_now ?? 0) > 0
              ? `${trip?.currency ?? ""} ${formatMoney(trip?.cancel_fee_now)}`
              : undefined
          }
          onWait={() => setConfirming(false)}
          onCancel={() => {
            setConfirming(false);
            router.push({ pathname: "/cancel-reason", params: { tripId: trip?.id ?? tripId } });
          }}
        />
      ) : null}
    </Animated.View>
  );
}

function moreActions(phase: TripPhase, trip: Trip | undefined, share: () => void, cancel: () => void) {
  const actions = [{ icon: "export-variant", label: "Share ride details", onPress: share }];
  if (trip?.driver?.phone) {
    const phone = trip.driver.phone;
    actions.push({
      icon: "message-text-outline",
      label: "Contact driver",
      onPress: () => Alert.alert("Contact driver", phone),
    });
  }
  if (canCancel(phase)) actions.push({ icon: "car-off", label: "Cancel ride", onPress: cancel });
  return actions;
}

/** After the rider cancels: back to the options with "Ride cancelled" (one tap to rebook). */
function useRebookWhenCancelled(phase: TripPhase) {
  const router = useRouter();
  const hasDraft = useBookingStore((s) => !!s.pickup && !!s.dropoff);
  const clearQuote = useBookingStore((s) => s.setQuote);
  useEffect(() => {
    if (phase !== "cancelled") return;
    if (!hasDraft) return router.replace("/");
    clearQuote(undefined); // quotes are single-use; price again
    router.replace({ pathname: "/book/options", params: { notice: "cancelled" } });
  }, [phase, router, hasDraft, clearQuote]);
}

/** When an assigned driver cancels, the server finds another; tell the rider once. */
function useDriverCancelledNotice(phase: TripPhase) {
  const prev = useRef(phase);
  const [text, setText] = useState<string>();
  useEffect(() => {
    if ((prev.current === "assigned" || prev.current === "arrived") && phase === "searching") {
      setText("Your driver cancelled. Finding you another driver…");
    }
    prev.current = phase;
  }, [phase]);
  return text ? { text, hide: () => setText(undefined) } : undefined;
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  pulse: { position: "absolute", top: 0, left: 0, right: 0, alignItems: "center", justifyContent: "center" },
  profile: { position: "absolute", left: 16 },
  sheet: {
    position: "absolute",
    left: 0,
    right: 0,
    bottom: 0,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    overflow: "hidden",
    elevation: 10,
    shadowColor: "#000000",
    shadowOpacity: 0.1,
    shadowRadius: 14,
  },
  content: { gap: 10 },
});
