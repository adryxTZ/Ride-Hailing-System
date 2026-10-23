/** CancelConfirmSheet — "Are you sure?" before cancelling a requested ride (video 1:12). */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { Pressable, StyleSheet, View } from "react-native";
import Animated, { FadeIn, FadeOut, SlideInDown, SlideOutDown } from "react-native-reanimated";

interface Props {
  bottom: number;
  /** "TZS 2,000" when cancelling now costs the cancellation fee; undefined when free. */
  fee?: string;
  onCancel: () => void;
  onWait: () => void;
}

export function CancelConfirmSheet({ bottom, fee, onCancel, onWait }: Props) {
  const { palette } = useTheme();
  const danger = palette.danger ?? palette.primary;
  return (
    <Animated.View entering={FadeIn} exiting={FadeOut} style={[StyleSheet.absoluteFill, styles.scrim]}>
      <Pressable style={StyleSheet.absoluteFill} onPress={onWait} accessibilityLabel="Close" />
      <Animated.View
        entering={SlideInDown.duration(280)}
        exiting={SlideOutDown}
        style={[styles.sheet, { backgroundColor: palette.background, paddingBottom: bottom + 20 }]}
      >
        <Pressable
          onPress={onWait}
          style={[styles.close, { backgroundColor: palette.surface }]}
          accessibilityLabel="Close"
        >
          <Icon name="close" size={20} color={palette.text} />
        </Pressable>
        <View style={[styles.art, { backgroundColor: withAlpha(danger, 0.12) }]}>
          <Icon name="steering" size={56} color={palette.text} />
          <View style={[styles.badge, { backgroundColor: danger, borderColor: palette.background }]}>
            <Icon name="close" size={22} color={palette.background} />
          </View>
        </View>
        <Text style={styles.title}>Are you sure?</Text>
        <Text variant="muted" style={styles.body}>
          {fee
            ? `Your driver is already on the way. Cancelling now costs ${fee}, added to your next ride.`
            : "Do you really want to cancel the ride? Rebooking may not get you to your destination more quickly."}
        </Text>
        <ScalePressable onPress={onCancel} style={[styles.btn, { backgroundColor: danger }]}>
          <Text style={[styles.btnText, { color: palette.background }]}>
            {fee ? `Cancel and pay ${fee}` : "Cancel ride"}
          </Text>
        </ScalePressable>
        <ScalePressable onPress={onWait} style={[styles.btn, { backgroundColor: palette.surface }]}>
          <Text style={styles.btnText}>Wait for driver</Text>
        </ScalePressable>
      </Animated.View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  scrim: { backgroundColor: "rgba(0,0,0,0.35)", justifyContent: "flex-end" },
  sheet: { borderTopLeftRadius: 24, borderTopRightRadius: 24, padding: 20, gap: 14, alignItems: "center" },
  close: {
    position: "absolute",
    top: 14,
    right: 14,
    width: 36,
    height: 36,
    borderRadius: 18,
    alignItems: "center",
    justifyContent: "center",
  },
  art: {
    width: 120,
    height: 120,
    borderRadius: 60,
    alignItems: "center",
    justifyContent: "center",
    marginTop: 8,
  },
  badge: {
    position: "absolute",
    right: 0,
    bottom: 4,
    width: 40,
    height: 40,
    borderRadius: 20,
    borderWidth: 3,
    alignItems: "center",
    justifyContent: "center",
  },
  title: { fontSize: 22, fontWeight: "700" },
  body: { textAlign: "center", paddingHorizontal: 8 },
  btn: { alignSelf: "stretch", height: 56, borderRadius: 28, alignItems: "center", justifyContent: "center" },
  btnText: { fontSize: 17, fontWeight: "700" },
});
