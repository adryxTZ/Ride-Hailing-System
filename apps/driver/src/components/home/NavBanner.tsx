/**
 * NavBanner — the turn-by-turn card floating over the map during a trip:
 * the next manoeuvre's arrow, distance and sentence, and a small "Then" row
 * for the one after. Presentational.
 */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeIn, FadeInDown, FadeOut } from "react-native-reanimated";
import type { NavState } from "@/hooks/useNavigation";

export function NavBanner({ nav, top }: { nav: NavState; top: number }) {
  const { palette } = useTheme();
  const ink = palette.on_primary ?? palette.background;
  return (
    <Animated.View
      entering={FadeInDown.duration(280)}
      exiting={FadeOut.duration(200)}
      style={[styles.card, { top, backgroundColor: palette.primary }]}
      accessibilityRole="header"
      accessibilityLabel={`${nav.now.text} in ${nav.distance}`}
    >
      <Animated.View key={nav.now.text} entering={FadeIn.duration(220)} style={styles.main}>
        <Icon name={nav.now.icon} size={44} color={ink} />
        <View style={styles.flex}>
          <Text style={[styles.distance, { color: ink }]}>{nav.distance}</Text>
          <Text style={[styles.text, { color: ink }]} numberOfLines={2}>
            {nav.now.text}
          </Text>
        </View>
      </Animated.View>
      {nav.then ? (
        <View style={[styles.then, { backgroundColor: withAlpha(ink, 0.14) }]}>
          <Text style={[styles.thenLabel, { color: ink }]}>Then</Text>
          <Icon name={nav.then.icon} size={18} color={ink} />
          <Text style={[styles.thenText, { color: ink }]} numberOfLines={1}>
            {nav.then.text}
          </Text>
        </View>
      ) : null}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  card: {
    position: "absolute",
    left: 12,
    right: 12,
    borderRadius: 18,
    padding: 14,
    gap: 10,
    shadowColor: "#000000",
    shadowOpacity: 0.12,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 4,
  },
  main: { flexDirection: "row", alignItems: "center", gap: 14 },
  flex: { flex: 1 },
  distance: { fontSize: 24, fontWeight: "800" },
  text: { fontSize: 17, fontWeight: "600" },
  then: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    borderRadius: 10,
    paddingHorizontal: 10,
    paddingVertical: 6,
  },
  thenLabel: { fontSize: 13, fontWeight: "700" },
  thenText: { flex: 1, fontSize: 13 },
});
