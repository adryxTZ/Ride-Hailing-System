/**
 * OptionsScreen — design 3 and the video: map with the route ("Pickup · N min",
 * "Dropoff · time"), a floating route bar, and a sheet that snaps between the
 * chosen ride, a half list and the full list with filters. Footer: personal /
 * work switch, "Confirm order" and schedule.
 */
import { Text, useTheme } from "@platform/ui";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useMemo, useRef, useState } from "react";
import {
  Alert,
  type LayoutChangeEvent,
  PanResponder,
  ScrollView,
  StyleSheet,
  useWindowDimensions,
  View,
} from "react-native";
import Animated, {
  FadeIn,
  FadeOut,
  LinearTransition,
  SlideInDown,
  SlideOutDown,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { PillButton } from "@/components/common/PillButton";
import { RoundButton } from "@/components/common/RoundButton";
import { OptionSkeleton } from "@/components/common/Skeleton";
import { Snackbar } from "@/components/common/Snackbar";
import { TaxiMap } from "@/components/map/TaxiMap";
import { FilterChips } from "@/components/taxi/FilterChips";
import { OptionCard } from "@/components/taxi/OptionCard";
import { ProfileSwitch } from "@/components/taxi/ProfileSwitch";
import { TopRouteBar } from "@/components/taxi/TopRouteBar";
import { useBack } from "@/hooks/useBack";
import { useNearbyVehicles } from "@/hooks/useNearbyVehicles";
import { usePaymentMethod } from "@/hooks/usePaymentMethod";
import { type OptionSort, type RideOption, useRideOptions } from "@/hooks/useRideOptions";
import { useRoute } from "@/hooks/useRoute";
import { useCreateTrip, useDues } from "@/hooks/useTrips";
import { clockIn, formatMoney } from "@/models/format";
import { routeLine } from "@/models/geo";
import { pickupLabel } from "@/models/schedule";
import { filledStops, useBookingStore } from "@/services/bookingStore";
import { fetchActiveTrip, type TripError } from "@/services/trips";

type Snap = "min" | "half" | "full";
const SNAPS: Snap[] = ["min", "half", "full"];
const SNAP_SHARE: Record<Snap, number> = { min: 0, half: 0.34, full: 0.58 }; // list height share of window
const SWIPE_PX = 24;
const TOP_BAR = 76;
const SKELETON_OPTIONS = 4;
const LEAVE_MS = 320; // let the sheet slide away before "Ride requested" appears

export function OptionsScreen() {
  const router = useRouter();
  const back = useBack();
  const insets = useSafeAreaInsets();
  const { height } = useWindowDimensions();
  const { palette } = useTheme();
  const { notice } = useLocalSearchParams<{ notice?: string }>();
  const booking = useBookingStore();
  const pay = usePaymentMethod();
  const [sort, setSort] = useState<OptionSort>("recommended");
  const [snap, setSnap] = useState<Snap>("half");
  const [sheetH, setSheetH] = useState(360);
  const [leaving, setLeaving] = useState(false);
  const [toast, setToast] = useState<string | undefined>(() => initialNotice(notice, booking.pickup?.source));
  const { options, recommendedKey, quote, pending, error } = useRideOptions(sort);
  const create = useCreateTrip();
  const { data: dues = 0 } = useDues();
  const surgedOption = quote?.options.find((o) => o.vehicle_class === booking.vehicleClass);
  const surged = !!surgedOption?.lines?.some((l) => l.code === "SURGE");
  const pickupEta = useRoadPickupEta(booking.vehicleClass);
  const selected = options.find((o) => o.key === booking.vehicleClass);
  const stopPoints = useMemo(() => filledStops(booking.stops).map((st) => st.point), [booking.stops]);
  const route = useMemo(
    () => routeLine(booking.pickup?.point, booking.dropoff?.point, quote?.route.polyline),
    [booking.pickup, booking.dropoff, quote?.route.polyline],
  );
  const pan = useSnapGesture(snap, setSnap);
  const hideToast = useCallback(() => setToast(undefined), []);

  const confirm = () => {
    if (!quote || !booking.vehicleClass) return;
    create.mutate(
      {
        quote_id: quote.quote_id,
        vehicle_class: booking.vehicleClass,
        payment_method: booking.paymentMethod,
        contacts: booking.contacts,
        scheduled_for: booking.scheduledFor,
        notes: booking.notes,
        stops: [
          { kind: "PICKUP", address: booking.pickup?.address },
          ...filledStops(booking.stops).map((st) => ({ kind: "WAYPOINT" as const, address: st.address })),
          ...(booking.dropoff ? [{ kind: "DROPOFF" as const, address: booking.dropoff.address }] : []),
        ],
      },
      {
        onSuccess: (trip) => {
          if (trip.status === "SCHEDULED") {
            return router.replace({ pathname: "/book/scheduled", params: { tripId: trip.id } });
          }
          setLeaving(true);
          setTimeout(
            () => router.replace({ pathname: "/book/requested", params: { tripId: trip.id } }),
            LEAVE_MS,
          );
        },
        // A ride is already in progress on this account: open it instead of failing.
        onError: async (e) => {
          if ((e as TripError).code !== "TRIP_ALREADY_ACTIVE") return;
          const active = await fetchActiveTrip().catch(() => null);
          if (active) router.replace({ pathname: "/book/requested", params: { tripId: active.id } });
        },
      },
    );
  };

  const list = snap === "min" ? (selected ? [selected] : []) : options;
  return (
    <View style={styles.root}>
      <TaxiMap
        centre={booking.pickup?.point ?? { lat: -6.79, lng: 39.21 }}
        insets={{ top: insets.top + TOP_BAR, bottom: sheetH }}
        pickup={booking.pickup?.point}
        pickupLabel={booking.scheduledFor ? "Pickup at" : "Pickup"}
        pickupValue={
          booking.scheduledFor
            ? pickupLabel(booking.scheduledFor)
            : (pickupEta ?? selected?.etaMin) != null
              ? `${pickupEta ?? selected?.etaMin} min`
              : undefined
        }
        dropoff={booking.dropoff?.point}
        dropoffEta={
          quote ? clockIn(quote.route.duration_s + (pickupEta ?? selected?.etaMin ?? 0) * 60) : undefined
        }
        route={route}
        stops={stopPoints}
        showsUser={false}
      />
      <TopRouteBar
        top={insets.top + 8}
        from={booking.pickup?.address ?? "Pickup"}
        to={booking.dropoff?.address ?? "Drop-off"}
        onClose={() => back()}
        onAdd={() => back()}
      />
      {snap === "full" ? (
        <Animated.View
          pointerEvents="none"
          entering={FadeIn}
          exiting={FadeOut}
          style={[StyleSheet.absoluteFill, styles.scrim]}
        />
      ) : null}
      {toast && !leaving ? (
        <Snackbar
          text={toast}
          bottom={sheetH + 12}
          onHide={hideToast}
          action={toast === SAVED_PICKUP ? "Edit" : undefined}
          onAction={() => back()}
        />
      ) : null}
      {leaving ? null : (
        <Animated.View
          entering={SlideInDown.duration(280)}
          exiting={SlideOutDown.duration(LEAVE_MS)}
          layout={LinearTransition.duration(240)}
          onLayout={(e: LayoutChangeEvent) => setSheetH(e.nativeEvent.layout.height)}
          style={[styles.sheet, { backgroundColor: palette.background, paddingBottom: insets.bottom + 12 }]}
        >
          <View {...pan.panHandlers} style={styles.handle}>
            <View style={[styles.grabber, { backgroundColor: palette.border }]} />
            {snap === "full" ? <FilterChips value={sort} onChange={setSort} /> : null}
          </View>
          {pending && !quote
            ? Array.from({ length: SKELETON_OPTIONS }, (_, i) => <OptionSkeleton key={i} />)
            : null}
          {error ? (
            <Text variant="error" style={styles.pad}>
              {error.message}
            </Text>
          ) : null}
          <ScrollView
            style={{ maxHeight: snap === "min" ? undefined : height * SNAP_SHARE[snap] }}
            contentContainerStyle={styles.list}
          >
            {list.map((o: RideOption) => (
              <OptionCard
                key={o.key}
                icon={o.icon}
                name={o.name}
                description={o.description}
                etaMin={o.key === booking.vehicleClass ? (pickupEta ?? o.etaMin) : o.etaMin}
                seats={o.seats}
                price={o.price}
                selected={o.key === booking.vehicleClass}
                recommended={o.key === recommendedKey}
                unavailable={!o.available}
                onPress={() => booking.setVehicleClass(o.key)}
              />
            ))}
          </ScrollView>
          <View style={[styles.footer, { borderTopColor: palette.border }]}>
            <ProfileSwitch
              value={booking.profile}
              onChange={booking.setProfile}
              label={`${booking.profile === "work" ? "Work ride" : "Personal ride"} · ${pay.method === "MOBILE_MONEY" ? "Mobile money" : "Cash"}`}
              onOpen={() => router.push("/account/payment")}
              compact={snap !== "full"}
            />
            {surged ? (
              <Text style={[styles.dues, { color: palette.primary }]} accessibilityRole="alert">
                Busy area right now · prices are slightly higher
              </Text>
            ) : null}
            {dues > 0 ? (
              <Text variant="muted" style={styles.dues}>
                {`${quote?.options[0]?.currency ?? ""} ${formatMoney(dues)} from an earlier cancellation is added to this ride.`}
              </Text>
            ) : null}
            {create.error ? <Text variant="error">{create.error.message}</Text> : null}
            <View style={styles.actions}>
              <PillButton
                title={
                  booking.scheduledFor ? `Schedule · ${pickupLabel(booking.scheduledFor)}` : "Confirm order"
                }
                onPress={confirm}
                loading={create.isPending}
                disabled={!selected}
                style={styles.flex}
              />
              {create.isPending ? null : (
                <RoundButton
                  icon="calendar-clock"
                  label="Schedule"
                  tone="primary"
                  size={62}
                  onPress={() => router.push("/book/schedule")}
                />
              )}
            </View>
          </View>
        </Animated.View>
      )}
    </View>
  );
}

/** Road minutes from the nearest car of the chosen class to the pickup (one cached route call). */
function useRoadPickupEta(vehicleClass?: string): number | undefined {
  const booking = useBookingStore();
  const { data: cars } = useNearbyVehicles(booking.pickup?.point, booking.serviceKey);
  const nearest = vehicleClass ? cars?.find((c) => c.vehicle_classes.includes(vehicleClass)) : undefined;
  const road = useRoute(nearest?.location, nearest ? booking.pickup?.point : undefined).data;
  return road ? Math.max(1, Math.round(road.duration_s / 60)) : undefined;
}

const SAVED_PICKUP = "A saved pickup location is being used";

function initialNotice(notice: string | undefined, pickupSource?: string): string | undefined {
  if (notice === "cancelled") return "Ride cancelled";
  return pickupSource === "saved" ? SAVED_PICKUP : undefined;
}

/** Vertical swipe on the sheet handle moves between snap points; a tap cycles min ↔ half. */
function useSnapGesture(snap: Snap, setSnap: (s: Snap) => void) {
  const current = useRef(snap);
  current.current = snap;
  return useMemo(
    () =>
      PanResponder.create({
        onStartShouldSetPanResponder: () => true,
        onPanResponderRelease: (_, g) => {
          const i = SNAPS.indexOf(current.current);
          if (g.dy < -SWIPE_PX) setSnap(SNAPS[Math.min(i + 1, SNAPS.length - 1)]);
          else if (g.dy > SWIPE_PX) setSnap(SNAPS[Math.max(i - 1, 0)]);
          else setSnap(current.current === "min" ? "half" : "min");
        },
      }),
    [setSnap],
  );
}

const styles = StyleSheet.create({
  dues: { fontSize: 13, textAlign: "center" },
  root: { flex: 1 },
  scrim: { backgroundColor: "rgba(0,0,0,0.32)" },
  sheet: {
    position: "absolute",
    left: 0,
    right: 0,
    bottom: 0,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    elevation: 10,
    shadowColor: "#000000",
    shadowOpacity: 0.1,
    shadowRadius: 14,
    shadowOffset: { width: 0, height: -4 },
  },
  handle: { paddingTop: 10, paddingBottom: 8, paddingHorizontal: 16, gap: 12 },
  grabber: { alignSelf: "center", width: 40, height: 5, borderRadius: 3 },
  pad: { paddingHorizontal: 16 },
  list: { paddingHorizontal: 12, gap: 2 },
  footer: { borderTopWidth: StyleSheet.hairlineWidth, paddingHorizontal: 16, paddingTop: 12, gap: 12 },
  actions: { flexDirection: "row", alignItems: "center", gap: 12 },
  flex: { flex: 1 },
});
