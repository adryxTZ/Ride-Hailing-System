/** TripRating — the rating form on a finished trip, or "You rated N★" once done. */
import { RatingForm } from "@platform/chat";
import { Icon, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";
import { useRating } from "@/hooks/useRating";
import { DRIVER_PRAISE, DRIVER_PROBLEMS } from "@/models/ratingAspects";

export function TripRating({ tripId, driverName }: { tripId: string; driverName?: string | null }) {
  const { palette } = useTheme();
  const { mine, loading, rate } = useRating(tripId, "RIDER");
  if (loading) return null;
  if (mine) {
    return (
      <Animated.View entering={FadeIn.duration(240)} style={styles.done}>
        <Icon name="star" size={22} color={palette.primary} />
        <Text>{`You rated this trip ${mine.score} of 5. Thank you!`}</Text>
      </Animated.View>
    );
  }
  return (
    <View>
      <RatingForm
        title={`How was your trip${driverName ? ` with ${driverName.split(" ")[0]}` : ""}?`}
        praise={DRIVER_PRAISE}
        problems={DRIVER_PROBLEMS}
        submitting={rate.isPending}
        onSubmit={(v) => rate.mutate(v)}
      />
      {rate.error ? <Text variant="error">{rate.error.message}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  done: { flexDirection: "row", alignItems: "center", gap: 8, justifyContent: "center", paddingVertical: 8 },
});
