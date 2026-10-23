/**
 * SplashReveal — the launch moment from the video (0:00–0:08): a full screen in
 * the brand colour with the brand name, then a circle opens from the centre and
 * reveals the app. Drawn as a ring whose border shrinks to nothing, so no
 * masking library is needed. Colour and name come from the server theme.
 */
import { Text, useTheme } from "@platform/ui";
import { useEffect, useState } from "react";
import { StyleSheet, useWindowDimensions, View } from "react-native";
import Animated, {
  Easing,
  FadeOut,
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useAppConfig } from "@/hooks/useAppConfig";

const HOLD_MS = 700; // brand moment once the theme is known
const MAX_WAIT_MS = 2500; // never block the app on a slow config call
const REVEAL_MS = 520;

export function SplashReveal() {
  const { palette } = useTheme();
  const { data: config } = useAppConfig();
  const { width, height } = useWindowDimensions();
  const [done, setDone] = useState(false);
  const [ready, setReady] = useState(false);
  const radius = Math.ceil(Math.hypot(width, height) / 2) + 2;
  const opening = useSharedValue(0);

  useEffect(() => {
    const id = setTimeout(() => setReady(true), config ? HOLD_MS : MAX_WAIT_MS);
    return () => clearTimeout(id);
  }, [config]);

  useEffect(() => {
    if (!ready) return;
    opening.value = withTiming(1, { duration: REVEAL_MS, easing: Easing.inOut(Easing.cubic) }, (finished) => {
      if (finished) runOnJS(setDone)(true);
    });
  }, [ready, opening]);

  const ring = useAnimatedStyle(() => ({ borderWidth: radius * (1 - opening.value) }));
  const word = useAnimatedStyle(() => ({
    opacity: 1 - opening.value * 3,
    transform: [{ scale: 1 + opening.value * 0.2 }],
  }));
  if (done) return null;
  return (
    <Animated.View exiting={FadeOut} pointerEvents="none" style={StyleSheet.absoluteFill}>
      <Animated.View
        style={[
          styles.ring,
          {
            width: radius * 2,
            height: radius * 2,
            borderRadius: radius,
            left: width / 2 - radius,
            top: height / 2 - radius,
          },
          { borderColor: palette.primary },
          ring,
        ]}
      />
      <View style={[StyleSheet.absoluteFill, styles.centre]}>
        <Animated.View style={word}>
          <Text style={[styles.word, { color: palette.on_primary ?? palette.background }]}>
            {config?.brand.name ?? ""}
          </Text>
        </Animated.View>
      </View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  ring: { position: "absolute" },
  centre: { alignItems: "center", justifyContent: "center" },
  word: { fontSize: 56, fontWeight: "800", letterSpacing: -1 },
});
