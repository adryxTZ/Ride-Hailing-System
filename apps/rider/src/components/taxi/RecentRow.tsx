/** RecentRow — a recent or saved place: icon in a soft square (home) or bare (route list), name, address, distance. */
import { Icon, ScalePressable, Text, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";

interface Props {
  icon: string;
  title: string;
  subtitle?: string;
  distanceKm?: number;
  /** "tile" = icon on a grey square (taxi home); "plain" = bare icon with a divider (route list). */
  variant?: "tile" | "plain";
  onPress: () => void;
}

export function RecentRow({ icon, title, subtitle, distanceKm, variant = "tile", onPress }: Props) {
  const { palette } = useTheme();
  const tile = variant === "tile";
  return (
    <ScalePressable onPress={onPress} style={styles.row}>
      <View style={[styles.icon, tile ? { backgroundColor: palette.surface } : styles.bare]}>
        <Icon name={icon} size={26} color={palette.text} />
      </View>
      <View style={[styles.body, tile ? null : { borderBottomColor: palette.border, ...styles.divided }]}>
        <View style={styles.texts}>
          <Text style={styles.title} numberOfLines={1}>
            {title}
          </Text>
          {subtitle ? (
            <Text variant="muted" numberOfLines={1}>
              {subtitle}
            </Text>
          ) : null}
        </View>
        {distanceKm != null ? <Text variant="muted">{distanceKm.toFixed(1)} km</Text> : null}
      </View>
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 14 },
  icon: { width: 56, height: 56, borderRadius: 12, alignItems: "center", justifyContent: "center" },
  bare: { width: 32, height: 56 },
  body: { flex: 1, flexDirection: "row", alignItems: "center", gap: 10, minHeight: 72 },
  divided: { borderBottomWidth: StyleSheet.hairlineWidth },
  texts: { flex: 1, gap: 3 },
  title: { fontSize: 17 },
});
