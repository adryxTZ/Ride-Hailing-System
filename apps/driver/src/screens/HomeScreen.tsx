/**
 * HomeScreen (driver) — map-first home. A full-screen map follows the driver; radar
 * rings pulse around the car while online; the header switch and the sheet's
 * "Go online" button flip presence. Sign-out lives in Account → Settings.
 */

import type { MapMarker } from "@platform/maps";
import { useTheme } from "@platform/ui";
import { useRouter } from "expo-router";
import { useState } from "react";
import { type LayoutChangeEvent, StyleSheet, View } from "react-native";
import Animated, { LinearTransition } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { RoundButton } from "@/components/common/RoundButton";
import { CarMarker } from "@/components/home/CarMarker";
import { DriverMap } from "@/components/home/DriverMap";
import { HoldPanel } from "@/components/home/HoldPanel";
import { NavBanner } from "@/components/home/NavBanner";
import { NextTripBanner } from "@/components/home/NextTripBanner";
import { OfferCard } from "@/components/home/OfferCard";
import { OfflinePanel } from "@/components/home/OfflinePanel";
import { OnlinePanel } from "@/components/home/OnlinePanel";
import { RadarRings } from "@/components/home/RadarRings";
import { SignupPanel } from "@/components/home/SignupPanel";
import { TopBar } from "@/components/home/TopBar";
import { TripPanel } from "@/components/home/TripPanel";
import { RateRiderPanel } from "@/components/rating/RateRiderPanel";
import { useAppConfig } from "@/hooks/useAppConfig";
import { DEFAULT_CENTRE, useCurrentLocation } from "@/hooks/useCurrentLocation";
import {
  useAcceptOffer,
  useDeclineOffer,
  useDriverTrip,
  useOffer,
  useRatePending,
  useTransition,
  useWallet,
} from "@/hooks/useDriverTrips";
import { useMyVehicle } from "@/hooks/useMyVehicle";
import { useNavigation } from "@/hooks/useNavigation";
import { useNextTrip } from "@/hooks/useNextTrip";
import { useOnboarding } from "@/hooks/useOnboarding";
import { useOnlineTimer } from "@/hooks/useOnlineTimer";
import { useDriverStatus, useLocationStream, useSetStatus } from "@/hooks/usePresence";
import { useProfile } from "@/hooks/useProfile";
import { useRoute } from "@/hooks/useRoute";
import { decodePolyline } from "@/models/geo";
import { progress } from "@/models/onboarding";
import { showsNextTrip } from "@/models/tripQueue";
import { isDriving, stepFor } from "@/models/tripSteps";
import { mainHold } from "@/models/wallet";
import type { Point } from "@/services/presence";
import type { Trip, TripStatus } from "@/services/trips";

const TAB_BAR_SPACE = 96; // floating pill tab bar + its margin
const TOP_BAR_SPACE = 72; // floating top bar height + gap; the nav banner sits below it
const SHEET_GUESS = 300; // used until the sheet measures itself

export function HomeScreen() {
  const { palette } = useTheme();
  const insets = useSafeAreaInsets();
  const router = useRouter();
  const { data: me } = useProfile();
  const myVehicle = useMyVehicle();
  const vehicleLabel = myVehicle.plate ? `${myVehicle.label} · ${myVehicle.plate}` : myVehicle.label;
  const { data: ob } = useOnboarding();
  const work = useWork();
  const ratePending = useRatePending();
  const presenceCtl = usePresenceControl(!!work.trip);
  const hold = mainHold(presenceCtl.holds);
  const { data: wallet } = useWallet();
  const brandCfg = useAppConfig().data?.brand;
  const road = useRoute(
    work.trip ? presenceCtl.centre : undefined,
    work.trip ? tripTarget(work.trip) : undefined,
    work.trip ? pendingStops(work.trip) : [],
  ).data;
  const nav = useNavigation(work.trip && isDriving(work.trip) ? road?.steps : undefined, presenceCtl.centre);
  const [sheetH, setSheetH] = useState(SHEET_GUESS);
  const [recenterKey, setRecenterKey] = useState(0);
  const onlineFor = useOnlineTimer(presenceCtl.online);
  const name = me?.full_name ?? undefined;

  return (
    <View style={[styles.root, { backgroundColor: palette.surface }]}>
      <DriverMap
        centre={presenceCtl.centre}
        bottomInset={sheetH}
        recenterKey={recenterKey}
        {...tripMapProps(work.trip, presenceCtl.centre, road?.polyline)}
      />
      {work.trip ? null : (
        <View pointerEvents="none" style={[styles.radar, { bottom: sheetH }]}>
          <RadarRings active={presenceCtl.online}>
            <CarMarker active={presenceCtl.online} />
          </RadarRings>
        </View>
      )}
      <TopBar
        name={name}
        online={presenceCtl.online}
        busy={presenceCtl.busy || !!work.trip}
        top={insets.top + 8}
        onAvatar={() => router.navigate("/account")}
        onToggle={presenceCtl.setOnline}
        canGoOnline={ob?.profile.status === "APPROVED" && !hold}
      />
      {nav ? <NavBanner nav={nav} top={insets.top + TOP_BAR_SPACE} /> : null}
      <RoundButton
        icon="crosshairs-gps"
        label="Re-centre map"
        onPress={() => setRecenterKey((k) => k + 1)}
        style={{ right: 16, bottom: sheetH + 14 }}
      />
      <Animated.View
        layout={LinearTransition.duration(240)}
        onLayout={(e: LayoutChangeEvent) => setSheetH(e.nativeEvent.layout.height)}
        style={[
          styles.sheet,
          { backgroundColor: palette.background, paddingBottom: TAB_BAR_SPACE + insets.bottom },
        ]}
      >
        <View style={[styles.grabber, { backgroundColor: palette.border }]} />
        {work.trip && showsNextTrip(work.trip) && work.next ? (
          <NextTripBanner next={work.next} onCancel={work.cancelNext} />
        ) : null}
        {work.trip && !work.offer?.queued ? (
          <TripPanel
            trip={work.trip}
            busy={work.stepping}
            error={work.error}
            onStep={work.step}
            onNoShow={work.noShow}
            road={road}
          />
        ) : work.offer ? (
          <OfferCard
            offer={work.offer}
            busy={work.answering}
            onAccept={work.accept}
            onDecline={work.decline}
          />
        ) : ratePending.trip ? (
          <RateRiderPanel
            tripId={ratePending.trip.id}
            riderName={ratePending.trip.rider?.name}
            onDone={ratePending.clear}
          />
        ) : !presenceCtl.online && ob && ob.profile.status !== "APPROVED" ? (
          <SignupPanel
            firstName={name?.split(" ")[0]}
            status={ob.profile.status}
            reason={ob.profile.status_reason ?? undefined}
            done={progress(ob).done}
            total={progress(ob).total}
            onOpen={() => router.push("/onboarding")}
          />
        ) : !presenceCtl.online && hold ? (
          <HoldPanel
            firstName={name?.split(" ")[0]}
            hold={hold}
            wallet={wallet}
            brand={brandCfg?.name}
            supportPhone={brandCfg?.support_phone}
            vehicleLabel={myVehicle.label}
            plate={myVehicle.plate}
            onPay={() => router.push("/(tabs)/earnings?pay=1")}
          />
        ) : presenceCtl.online ? (
          <OnlinePanel
            name={name}
            vehicleLabel={vehicleLabel}
            onlineFor={onlineFor}
            error={presenceCtl.error}
          />
        ) : (
          <OfflinePanel
            firstName={name?.split(" ")[0]}
            vehicleLabel={myVehicle.label}
            plate={myVehicle.plate}
            busy={presenceCtl.busy}
            error={presenceCtl.error}
            onGoOnline={() => presenceCtl.setOnline(true)}
          />
        )}
      </Animated.View>
    </View>
  );
}

/** The live offer and assigned trip (socket-driven), with the driver's actions on them. */
function useWork() {
  const { data: offer } = useOffer();
  const { data: trip } = useDriverTrip();
  const accept = useAcceptOffer();
  const decline = useDeclineOffer();
  const transition = useTransition();
  const { data: next } = useNextTrip(!!trip);
  const error = (accept.error ?? transition.error)?.message;
  return {
    offer: offer ?? undefined,
    trip: trip ?? undefined,
    /** The trip accepted while finishing this one (banner). */
    next: next ?? undefined,
    answering: accept.isPending || decline.isPending,
    stepping: transition.isPending,
    error,
    accept: () => offer && accept.mutate(offer.trip_id),
    decline: () => offer && decline.mutate(offer.trip_id),
    step: (to: TripStatus) => trip && transition.mutate({ id: trip.id, to }),
    /** The driver gives up the queued next trip: it goes back to matching. */
    cancelNext: () => next && transition.mutate({ id: next.trip_id, to: "CANCELLED_BY_DRIVER" }),
    noShow: () =>
      trip && transition.mutate({ id: trip.id, to: "CANCELLED_BY_DRIVER", reason: "rider_no_show" }),
  };
}

/** On a trip the map shows the car, the stop being driven to and the line between them. */
/** Stops still ahead on the way to the drop-off (none while heading to pickup). */
function pendingStops(trip: Trip): Point[] {
  if (stepFor(trip).target === "PICKUP") return [];
  return trip.stops.filter((st) => st.kind === "WAYPOINT" && !st.arrived_at).map((st) => st.location);
}

function tripTarget(trip: Trip): Point | undefined {
  return trip.stops.find((st) => st.kind === stepFor(trip).target)?.location;
}

function tripMapProps(trip: Trip | undefined, here: Point, polyline?: string) {
  if (!trip) return {};
  const target = tripTarget(trip);
  if (!target) return {};
  const markers: MapMarker[] = [
    { id: "me", kind: "car", at: here },
    {
      id: "target",
      kind: stepFor(trip).target === "PICKUP" ? "pickup" : "dropoff",
      at: target,
      label: stepFor(trip).target === "PICKUP" ? "Pickup" : "Drop-off",
    },
  ];
  const via = pendingStops(trip);
  via.forEach((st, i) => {
    markers.push({ id: `stop-${i}`, kind: "stop", at: st });
  });
  const line = polyline ? decodePolyline(polyline) : [here, ...via, target];
  return { markers, route: line, fitTo: [here, ...via, target] };
}

/** Presence state + the go online/offline action, kept out of the view. */
function usePresenceControl(onTrip: boolean) {
  const { point, denied, mocked } = useCurrentLocation();
  const status = useDriverStatus();
  const mutation = useSetStatus();
  const [error, setError] = useState<string>();
  const online = status.data?.online ?? false;
  useLocationStream(online, onTrip);

  const setOnline = async (next: boolean) => {
    setError(undefined);
    try {
      if (!next) return void (await mutation.mutateAsync({ online: false }));
      if (!point) throw new Error(denied ? "Allow location to go online" : "Waiting for your location…");
      // The server takes the service and class from the approved vehicle.
      await mutation.mutateAsync({ online: true, location: point, mocked });
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return {
    online,
    holds: status.data?.holds ?? [],
    busy: mutation.isPending || status.isLoading,
    error,
    centre: status.data?.presence?.location ?? point ?? DEFAULT_CENTRE,
    setOnline: (next: boolean) => void setOnline(next),
  };
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  radar: { position: "absolute", top: 0, left: 0, right: 0, alignItems: "center", justifyContent: "center" },
  sheet: {
    position: "absolute",
    left: 0,
    right: 0,
    bottom: 0,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    paddingHorizontal: 20,
    paddingTop: 10,
    gap: 12,
    shadowColor: "#000000",
    shadowOpacity: 0.08,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: -4 },
    elevation: 8,
  },
  grabber: { alignSelf: "center", width: 40, height: 4, borderRadius: 2, marginBottom: 6 },
});
