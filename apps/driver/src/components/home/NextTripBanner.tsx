/**
 * NextTripBanner — compact "Next trip queued" call-out above the trip panel while
 * the driver finishes the current trip: the next pickup, the rider's first name
 * and how far it is from this drop-off. Tapping it offers to give the trip up
 * (it goes back to matching).
 */
import { Banner } from "@platform/ui";
import { Alert } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { nextTripLine, nextTripPlace } from "@/models/tripQueue";
import type { NextTrip } from "@/services/tripQueue";

const ENTER_MS = 260;

export function NextTripBanner({ next, onCancel }: { next: NextTrip; onCancel: () => void }) {
  const confirm = () =>
    Alert.alert("Cancel your next trip?", "The rider will be matched with another driver.", [
      { text: "Keep it", style: "cancel" },
      { text: "Cancel next trip", style: "destructive", onPress: onCancel },
    ]);
  return (
    <Animated.View entering={FadeInDown.duration(ENTER_MS)}>
      <Banner
        icon="car-clock"
        title={`Next trip queued · ${nextTripPlace(next)}`}
        highlight={nextTripLine(next)}
        onPress={confirm}
      />
    </Animated.View>
  );
}
