/**
 * SafetySheet — opened from the trip's Safety pill: share the trip live, alert
 * the trusted contacts (SOS), or call the emergency number.
 */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, Alert, Linking, Pressable, StyleSheet, View } from "react-native";
import Animated, { FadeIn, FadeOut, SlideInDown, SlideOutDown } from "react-native-reanimated";

interface Props {
  bottom: number;
  emergencyPhone?: string;
  hasContacts: boolean;
  sharing: boolean;
  alerting: boolean;
  status?: string;
  onShare: () => void;
  onSos: () => void;
  onAddContacts: () => void;
  onClose: () => void;
}

export function SafetySheet(p: Props) {
  const { palette } = useTheme();
  const danger = palette.danger ?? palette.primary;
  const confirmSos = () => {
    if (!p.hasContacts) return p.onAddContacts();
    Alert.alert(
      "Alert your trusted contacts?",
      "They get a text with a link to follow this trip live. Our team is alerted too.",
      [
        { text: "Cancel", style: "cancel" },
        { text: "Send alert", style: "destructive", onPress: p.onSos },
      ],
    );
  };
  return (
    <Animated.View entering={FadeIn} exiting={FadeOut} style={[StyleSheet.absoluteFill, styles.scrim]}>
      <Pressable style={StyleSheet.absoluteFill} onPress={p.onClose} accessibilityLabel="Close" />
      <Animated.View
        entering={SlideInDown.duration(280)}
        exiting={SlideOutDown}
        style={[styles.sheet, { backgroundColor: palette.background, paddingBottom: p.bottom + 20 }]}
      >
        <Text style={styles.title}>Safety</Text>
        <Row
          icon="share-variant"
          title="Share trip status"
          subtitle="Send a live link of this trip"
          busy={p.sharing}
          onPress={p.onShare}
        />
        <Row
          icon="account-alert"
          title={p.hasContacts ? "Alert my trusted contacts" : "Add trusted contacts"}
          subtitle={
            p.hasContacts
              ? "Texts them your live trip and alerts our team"
              : "People we alert when you press SOS"
          }
          busy={p.alerting}
          onPress={confirmSos}
        />
        {p.emergencyPhone ? (
          <ScalePressable
            accessibilityLabel={`Call emergency ${p.emergencyPhone}`}
            onPress={() => void Linking.openURL(`tel:${p.emergencyPhone}`)}
            style={[styles.emergency, { backgroundColor: danger }]}
          >
            <Icon name="phone-alert" size={24} color={palette.background} />
            <Text
              style={[styles.emergencyText, { color: palette.background }]}
            >{`Call emergency services · ${p.emergencyPhone}`}</Text>
          </ScalePressable>
        ) : null}
        {p.status ? <Text style={[styles.status, { color: palette.primary }]}>{p.status}</Text> : null}
      </Animated.View>
    </Animated.View>
  );
}

function Row(r: { icon: string; title: string; subtitle: string; busy?: boolean; onPress: () => void }) {
  const { palette } = useTheme();
  return (
    <ScalePressable
      accessibilityLabel={r.title}
      onPress={r.onPress}
      disabled={r.busy}
      style={[styles.row, { backgroundColor: palette.surface }]}
    >
      <View style={[styles.icon, { backgroundColor: withAlpha(palette.primary, 0.12) }]}>
        <Icon name={r.icon} size={24} color={palette.primary} />
      </View>
      <View style={styles.flex}>
        <Text style={styles.rowTitle}>{r.title}</Text>
        <Text variant="muted">{r.subtitle}</Text>
      </View>
      {r.busy ? (
        <ActivityIndicator color={palette.primary} />
      ) : (
        <Icon name="chevron-right" size={22} color={palette.muted ?? palette.text} />
      )}
    </ScalePressable>
  );
}

const styles = StyleSheet.create({
  scrim: { backgroundColor: "rgba(0,0,0,0.35)", justifyContent: "flex-end" },
  sheet: { borderTopLeftRadius: 24, borderTopRightRadius: 24, padding: 20, gap: 12 },
  title: { fontSize: 22, fontWeight: "700", marginBottom: 4 },
  row: { flexDirection: "row", alignItems: "center", gap: 12, borderRadius: 18, padding: 14, minHeight: 72 },
  icon: { width: 44, height: 44, borderRadius: 22, alignItems: "center", justifyContent: "center" },
  flex: { flex: 1, gap: 2 },
  rowTitle: { fontSize: 16, fontWeight: "700" },
  emergency: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 10,
    height: 56,
    borderRadius: 28,
    marginTop: 4,
  },
  emergencyText: { fontSize: 16, fontWeight: "700" },
  status: { textAlign: "center", fontWeight: "600" },
});
