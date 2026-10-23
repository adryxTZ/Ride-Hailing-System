/**
 * ContactCard — a titled card collecting one person's name and phone number
 * (the passenger or sender at the pickup, or the receiver at the drop-off).
 * When `whoOptions` is given the rider first confirms whether it is themselves
 * or someone else; the fields only show for "someone else".
 */
import { Icon, ScalePressable, Text, TextField, useTheme } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown, LinearTransition } from "react-native-reanimated";

export interface ContactValue {
  name: string;
  phone: string;
}

interface Props {
  icon: string;
  title: string;
  subtitle: string;
  value?: ContactValue;
  onChange: (v: ContactValue) => void;
  /** Labels for the "me" / "someone else" choice; omit when someone else is always required. */
  whoOptions?: { me: string; other: string };
  forOther?: boolean;
  onForOther?: (other: boolean) => void;
  index?: number;
}

const ENTER_STAGGER_MS = 60;

export function ContactCard(p: Props) {
  const { palette } = useTheme();
  const showFields = !p.whoOptions || p.forOther;
  const v = p.value ?? { name: "", phone: "" };
  return (
    <Animated.View
      entering={FadeInDown.delay((p.index ?? 0) * ENTER_STAGGER_MS)}
      layout={LinearTransition}
      style={[styles.card, { backgroundColor: palette.surface, borderColor: palette.border }]}
    >
      <View style={styles.head}>
        <View style={[styles.badge, { backgroundColor: palette.background }]}>
          <Icon name={p.icon} size={22} color={palette.primary} />
        </View>
        <View style={{ flex: 1 }}>
          <Text style={styles.title}>{p.title}</Text>
          <Text variant="muted">{p.subtitle}</Text>
        </View>
      </View>
      {p.whoOptions ? (
        <View style={styles.choice}>
          <Choice label={p.whoOptions.me} active={!p.forOther} onPress={() => p.onForOther?.(false)} />
          <Choice label={p.whoOptions.other} active={!!p.forOther} onPress={() => p.onForOther?.(true)} />
        </View>
      ) : null}
      {showFields ? (
        <Animated.View entering={FadeInDown} style={styles.fields}>
          <TextField
            label="Full name"
            value={v.name}
            onChangeText={(name) => p.onChange({ ...v, name })}
            autoCapitalize="words"
            placeholder="Name"
          />
          <TextField
            label="Phone number"
            value={v.phone}
            onChangeText={(phone) => p.onChange({ ...v, phone })}
            keyboardType="phone-pad"
            placeholder="07XX XXX XXX"
          />
        </Animated.View>
      ) : null}
    </Animated.View>
  );
}

/** One side of the me / someone-else switch. */
function Choice({ label, active, onPress }: { label: string; active: boolean; onPress: () => void }) {
  const { palette } = useTheme();
  return (
    <ScalePressable
      onPress={onPress}
      style={[
        styles.pill,
        {
          backgroundColor: active ? palette.primary : palette.background,
          borderColor: active ? palette.primary : palette.border,
        },
      ]}
    >
      <Text
        style={[
          styles.pillText,
          { color: active ? (palette.on_primary ?? palette.background) : palette.text },
        ]}
      >
        {label}
      </Text>
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  card: {
    borderRadius: 18,
    borderWidth: 1,
    padding: 16,
    gap: 14,
    shadowColor: "#000000",
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  head: { flexDirection: "row", alignItems: "center", gap: 12 },
  badge: { width: 44, height: 44, borderRadius: 22, alignItems: "center", justifyContent: "center" },
  title: { fontSize: 17, fontWeight: "700" },
  choice: { flexDirection: "row", gap: 10 },
  pill: {
    flex: 1,
    height: 44,
    borderRadius: 22,
    borderWidth: 1,
    alignItems: "center",
    justifyContent: "center",
  },
  pillText: { fontSize: 15, fontWeight: "600" },
  fields: { gap: 12 },
});
