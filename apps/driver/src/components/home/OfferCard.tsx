/**
 * OfferCard — a ride offer (driver board 1, middle screen): who wants a ride, the
 * fare and distance, pickup and drop-off, a countdown, Decline / Accept.
 */

import { Avatar, Button, Text, useTheme, withAlpha } from "@platform/ui";
import { useEffect } from "react";
import { type LayoutChangeEvent, Pressable, StyleSheet, View } from "react-native";
import Animated, {
  Easing,
  FadeInDown,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { isPackage } from "@/models/tripSteps";
import type { Offer } from "@/services/trips";

interface Props {
  offer: Offer;
  busy: boolean;
  onAccept: () => void;
  onDecline: () => void;
}

const formatKm = (m?: number) => (m == null ? "" : `${(m / 1000).toFixed(1)} km`);

export function OfferCard({ offer, busy, onAccept, onDecline }: Props) {
  const { palette } = useTheme();
  const name = offer.rider_name ?? "A rider";
  const pkg = isPackage(offer);
  return (
    <Animated.View entering={FadeInDown.duration(280)} style={styles.wrap}>
      <Text style={[styles.hey, { color: palette.success ?? palette.primary }]}>
        {pkg ? `Package delivery for ${name}` : `${name} wants to ride with you`}
      </Text>
      <View style={styles.row}>
        <Avatar name={name} size={44} />
        <View style={styles.flex}>
          <Text style={styles.name}>{name}</Text>
          <Text style={{ color: palette.primary }}>
            {offer.payment_method === "CASH" ? "Cash" : offer.payment_method}
          </Text>
        </View>
        <View style={styles.right}>
          <Text
            style={styles.price}
          >{`${offer.currency} ${Math.round(Number(offer.fare)).toLocaleString("en-US")}`}</Text>
          <Text variant="muted">{formatKm(offer.trip_distance_m)}</Text>
        </View>
      </View>
      <View style={styles.stops}>
        <Stop
          label={`Pickup · ${formatKm(offer.pickup_distance_m)} away`}
          address={offer.pickup.address}
          dark
        />
        {offer.stops_count ? (
          <Text variant="muted">{`+ ${offer.stops_count} stop${offer.stops_count > 1 ? "s" : ""} on the way`}</Text>
        ) : null}
        {offer.dropoff ? <Stop label="Drop-off" address={offer.dropoff.address} /> : null}
      </View>
      <Countdown expiresAt={offer.expires_at} total={offer.ttl_s} />
      <View style={styles.buttons}>
        <Pressable
          onPress={onDecline}
          disabled={busy}
          style={[styles.decline, { borderColor: palette.primary }]}
        >
          <Text style={[styles.declineText, { color: palette.primary }]}>Decline</Text>
        </Pressable>
        <View style={styles.flex}>
          <Button title="Accept" onPress={onAccept} loading={busy} />
        </View>
      </View>
    </Animated.View>
  );
}

function Stop({ label, address, dark }: { label: string; address?: string; dark?: boolean }) {
  const { palette } = useTheme();
  return (
    <View style={styles.stop}>
      <View style={[styles.dot, { borderColor: dark ? palette.text : palette.primary }]} />
      <View style={styles.flex}>
        <Text variant="muted" style={styles.small}>
          {label}
        </Text>
        <Text numberOfLines={1}>{address ?? "Pinned location"}</Text>
      </View>
    </View>
  );
}

/** A bar that empties until the offer expires. */
function Countdown({ expiresAt, total }: { expiresAt: string; total: number }) {
  const { palette } = useTheme();
  const width = useSharedValue(0);
  const left = useSharedValue(1);
  useEffect(() => {
    const ms = Math.max(0, new Date(expiresAt).getTime() - Date.now());
    left.value = Math.min(1, ms / Math.max(1, total * 1000));
    left.value = withTiming(0, { duration: ms, easing: Easing.linear });
  }, [expiresAt, total, left]);
  const bar = useAnimatedStyle(() => ({ width: width.value * left.value }));
  return (
    <View
      onLayout={(e: LayoutChangeEvent) => {
        width.value = e.nativeEvent.layout.width;
      }}
      style={[styles.track, { backgroundColor: withAlpha(palette.primary, 0.15) }]}
    >
      <Animated.View style={[styles.fill, { backgroundColor: palette.primary }, bar]} />
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 14 },
  hey: { fontSize: 15, fontWeight: "600" },
  row: { flexDirection: "row", alignItems: "center", gap: 12 },
  flex: { flex: 1 },
  name: { fontSize: 17, fontWeight: "600" },
  right: { alignItems: "flex-end" },
  price: { fontSize: 20, fontWeight: "700" },
  stops: { gap: 12 },
  stop: { flexDirection: "row", alignItems: "center", gap: 12 },
  dot: { width: 14, height: 14, borderRadius: 7, borderWidth: 4 },
  small: { fontSize: 12 },
  track: { height: 6, borderRadius: 3, overflow: "hidden" },
  fill: { height: 6, borderRadius: 3 },
  buttons: { flexDirection: "row", gap: 12, alignItems: "center" },
  decline: {
    flex: 1,
    height: 52,
    borderRadius: 12,
    borderWidth: 1.5,
    alignItems: "center",
    justifyContent: "center",
  },
  declineText: { fontSize: 16, fontWeight: "700" },
});
