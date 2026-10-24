/** PrivacyActionTile — one tappable card on the privacy screen ("Download my data", "Delete my account"). */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { cardStyles, ENTER_MS, ENTER_STAGGER_MS } from "./cardStyle";

interface Props {
  icon: string;
  title: string;
  subtitle: string;
  /** Position in the screen's entrance stagger. */
  index: number;
  /** Paints the icon in the danger colour (deleting the account). */
  destructive?: boolean;
  busy?: boolean;
  disabled?: boolean;
  onPress: () => void;
}

export function PrivacyActionTile({
  icon,
  title,
  subtitle,
  index,
  destructive,
  busy,
  disabled,
  onPress,
}: Props) {
  const { palette } = useTheme();
  const tint = destructive ? (palette.danger ?? palette.primary) : palette.primary;
  const inactive = disabled || busy;
  return (
    <Animated.View entering={FadeInDown.delay(index * ENTER_STAGGER_MS).duration(ENTER_MS)}>
      <ScalePressable
        accessibilityRole="button"
        accessibilityLabel={title}
        accessibilityState={{ disabled: !!inactive, busy: !!busy }}
        onPress={onPress}
        disabled={inactive}
        style={[
          cardStyles.card,
          styles.row,
          { backgroundColor: palette.surface, shadowColor: palette.text, opacity: disabled ? 0.6 : 1 },
        ]}
      >
        <View style={[styles.badge, { backgroundColor: withAlpha(tint, 0.12) }]}>
          {busy ? <ActivityIndicator color={tint} /> : <Icon name={icon} size={22} color={tint} />}
        </View>
        <View style={styles.body}>
          <Text style={styles.title}>{title}</Text>
          <Text variant="muted">{subtitle}</Text>
        </View>
        <Icon name="chevron-right" size={22} color={palette.muted ?? palette.text} />
      </ScalePressable>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 14 },
  badge: { width: 44, height: 44, borderRadius: 22, alignItems: "center", justifyContent: "center" },
  body: { flex: 1, gap: 2 },
  title: { fontSize: 16, fontWeight: "700" },
});
