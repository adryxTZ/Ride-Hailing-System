import { type PressableProps, Pressable as RNPressable } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";

const PRESS_MS = 110; // quick, no overshoot

const AnimatedPressable = Animated.createAnimatedComponent(RNPressable);

/**
 * ScalePressable — a Pressable that springs to 97% while pressed. Use it for
 * cards and tiles so every tap in the app has the same tactile response.
 */
export function ScalePressable({ style, onPressIn, onPressOut, ...rest }: PressableProps) {
  const scale = useSharedValue(1);
  const animated = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  return (
    <AnimatedPressable
      {...rest}
      onPressIn={(e) => {
        scale.value = withTiming(0.97, { duration: PRESS_MS });
        onPressIn?.(e);
      }}
      onPressOut={(e) => {
        scale.value = withTiming(1, { duration: PRESS_MS });
        onPressOut?.(e);
      }}
      style={[typeof style === "function" ? undefined : style, animated]}
    />
  );
}
