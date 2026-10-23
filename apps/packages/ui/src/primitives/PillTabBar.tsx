import { Pressable, StyleSheet, View } from "react-native";
import Animated, { FadeIn, LinearTransition } from "react-native-reanimated";
import { useTheme } from "../ThemeProvider";
import { Icon } from "./Icon";
import { Text } from "./Text";

/** Structural subset of React Navigation's BottomTabBarProps, so this package does not pin a navigation version. */
export interface PillTabBarProps {
  state: { index: number; routes: { key: string; name: string }[] };
  descriptors: Record<string, { options: { title?: string; tabBarIconName?: string } }>;
  navigation: {
    emit: (e: { type: "tabPress"; target: string; canPreventDefault: true }) => { defaultPrevented: boolean };
    navigate: (name: string) => void;
  };
}

/**
 * PillTabBar — floating rounded tab bar (concept boards): dark pill container,
 * the active tab expands into a brand-coloured pill with icon + label. The
 * expansion animates with a layout transition.
 */
export function PillTabBar({ state, descriptors, navigation }: PillTabBarProps) {
  const { palette } = useTheme();
  return (
    <View style={styles.wrap} pointerEvents="box-none">
      <View style={[styles.pill, { backgroundColor: palette.text }]}>
        {state.routes.map((route, index) => {
          const { options } = descriptors[route.key] ?? { options: {} };
          const active = state.index === index;
          const label = options.title ?? route.name;
          const icon = options.tabBarIconName ?? "circle-outline";
          return (
            <Animated.View key={route.key} layout={LinearTransition.duration(240)}>
              <Pressable
                accessibilityRole="tab"
                accessibilityState={{ selected: active }}
                onPress={() => {
                  const event = navigation.emit({
                    type: "tabPress",
                    target: route.key,
                    canPreventDefault: true,
                  });
                  if (!active && !event.defaultPrevented) navigation.navigate(route.name);
                }}
                style={[styles.item, active && { backgroundColor: palette.primary, paddingHorizontal: 18 }]}
              >
                <Icon
                  name={icon}
                  size={24}
                  color={active ? (palette.on_primary ?? "#FFFFFF") : palette.background}
                />
                {active ? (
                  <Animated.View entering={FadeIn.duration(180)}>
                    <Text style={[styles.label, { color: palette.on_primary ?? "#FFFFFF" }]}>{label}</Text>
                  </Animated.View>
                ) : null}
              </Pressable>
            </Animated.View>
          );
        })}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { position: "absolute", left: 0, right: 0, bottom: 18, alignItems: "center" },
  pill: { flexDirection: "row", alignItems: "center", gap: 6, padding: 6, borderRadius: 999 },
  item: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    height: 52,
    minWidth: 52,
    paddingHorizontal: 14,
    borderRadius: 999,
    justifyContent: "center",
  },
  label: { fontSize: 15, fontWeight: "700" },
});
