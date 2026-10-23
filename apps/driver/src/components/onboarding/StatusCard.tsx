/** StatusCard — where the registration stands: icon, headline, explanation, reviewer's note and progress. */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";

interface Props {
  icon: string;
  title: string;
  body: string;
  reason?: string;
  done: number;
  total: number;
  showProgress: boolean;
}

export function StatusCard(p: Props) {
  const { palette } = useTheme();
  const share = p.total > 0 ? p.done / p.total : 0;
  return (
    <Animated.View
      entering={FadeInDown.duration(280)}
      style={[
        styles.card,
        { backgroundColor: withAlpha(palette.primary, 0.08), borderColor: withAlpha(palette.primary, 0.2) },
      ]}
      accessibilityRole="summary"
    >
      <View style={styles.head}>
        <Icon name={p.icon} size={30} color={palette.primary} />
        <Text style={styles.title}>{p.title}</Text>
      </View>
      <Text variant="muted">{p.body}</Text>
      {p.reason ? (
        <View
          style={[
            styles.reason,
            { backgroundColor: palette.background, borderColor: palette.danger ?? palette.accent },
          ]}
        >
          <Text style={{ color: palette.danger ?? palette.accent, fontWeight: "600" }}>{p.reason}</Text>
        </View>
      ) : null}
      {p.showProgress ? (
        <View accessibilityLabel={`${p.done} of ${p.total} steps done`}>
          <View style={[styles.track, { backgroundColor: withAlpha(palette.primary, 0.15) }]}>
            <View
              style={[
                styles.fill,
                { width: `${Math.round(share * 100)}%`, backgroundColor: palette.primary },
              ]}
            />
          </View>
          <Text variant="muted" style={styles.count}>{`${p.done} of ${p.total} steps done`}</Text>
        </View>
      ) : null}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: 18, borderWidth: 1, padding: 16, gap: 10 },
  head: { flexDirection: "row", alignItems: "center", gap: 10 },
  title: { fontSize: 20, fontWeight: "700", flex: 1 },
  reason: { borderWidth: 1, borderRadius: 12, padding: 12 },
  track: { height: 8, borderRadius: 4, overflow: "hidden" },
  fill: { height: 8, borderRadius: 4 },
  count: { marginTop: 6, fontSize: 13 },
});
