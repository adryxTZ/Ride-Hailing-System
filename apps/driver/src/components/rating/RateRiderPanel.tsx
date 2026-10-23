/** RateRiderPanel — after a trip ends, rate the rider (or skip) before taking the next one. */
import { RatingForm } from "@platform/chat";
import { Text } from "@platform/ui";
import { useEffect } from "react";
import { View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { useRating } from "@/hooks/useRating";
import { RIDER_PRAISE, RIDER_PROBLEMS } from "@/models/ratingAspects";

export function RateRiderPanel({
  tripId,
  riderName,
  onDone,
}: {
  tripId: string;
  riderName?: string | null;
  onDone: () => void;
}) {
  const { mine, rate } = useRating(tripId, "DRIVER");
  useEffect(() => {
    if (mine || rate.isSuccess) onDone();
  }, [mine, rate.isSuccess, onDone]);
  return (
    <Animated.View entering={FadeInDown.duration(280)}>
      <View>
        <RatingForm
          title={`Rate ${riderName?.split(" ")[0] ?? "the rider"}`}
          praise={RIDER_PRAISE}
          problems={RIDER_PROBLEMS}
          submitting={rate.isPending}
          onSubmit={(v) => rate.mutate(v)}
          onSkip={onDone}
        />
        {rate.error ? <Text variant="error">{rate.error.message}</Text> : null}
      </View>
    </Animated.View>
  );
}
