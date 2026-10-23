/** OutcomeCard — the end of a trip: completed (fare, Done) or no driver found (Try again). */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import { PillButton } from "@/components/common/PillButton";
import { SheetCard } from "./SheetCard";

interface Props {
  icon: string;
  title: string;
  body: string;
  action?: string;
  onAction?: () => void;
  secondary?: { label: string; onPress: () => void };
  children?: React.ReactNode;
}

export function OutcomeCard({ icon, title, body, action, onAction, secondary, children }: Props) {
  const { palette } = useTheme();
  return (
    <SheetCard style={styles.card}>
      <View style={[styles.badge, { backgroundColor: withAlpha(palette.primary, 0.12) }]}>
        <Icon name={icon} size={36} color={palette.primary} />
      </View>
      <Text style={styles.title}>{title}</Text>
      <Text variant="muted" style={styles.body}>
        {body}
      </Text>
      {children}
      {action && onAction ? <PillButton title={action} onPress={onAction} style={styles.button} /> : null}
      {secondary ? (
        <Text onPress={secondary.onPress} style={[styles.link, { color: palette.primary }]}>
          {secondary.label}
        </Text>
      ) : null}
    </SheetCard>
  );
}

const styles = StyleSheet.create({
  card: { alignItems: "center", paddingTop: 28 },
  badge: { width: 76, height: 76, borderRadius: 38, alignItems: "center", justifyContent: "center" },
  title: { fontSize: 24, fontWeight: "700", textAlign: "center" },
  body: { textAlign: "center" },
  button: { alignSelf: "stretch", marginTop: 8 },
  link: { fontSize: 16, fontWeight: "600", paddingVertical: 8 },
});
