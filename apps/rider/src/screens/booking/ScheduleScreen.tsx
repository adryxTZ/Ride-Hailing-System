/**
 * ScheduleScreen — "When should we pick you up?": a day, then a time on the
 * 15-minute grid (30 minutes to 7 days ahead). "Set pickup time" stores it on
 * the booking; "Ride now" clears it. From Home it continues to the route.
 */
import { Header, Text, useTheme } from "@platform/ui";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useMemo, useState } from "react";
import { ScrollView, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { PillButton } from "@/components/common/PillButton";
import { ChoiceChip } from "@/components/schedule/ChoiceChip";
import { useBack } from "@/hooks/useBack";
import { clock, dayOptions, pickupLabel, timeSlots } from "@/models/schedule";
import { useBookingStore } from "@/services/bookingStore";

const STAGGER_MS = 20;
const STAGGER_CAP = 12;

export function ScheduleScreen() {
  const router = useRouter();
  const back = useBack();
  const { from } = useLocalSearchParams<{ from?: string }>();
  const insets = useSafeAreaInsets();
  const { palette } = useTheme();
  const booking = useBookingStore();
  const now = useMemo(() => new Date(), []);
  const days = useMemo(() => dayOptions(now), [now]);
  const current = booking.scheduledFor ? new Date(booking.scheduledFor) : undefined;
  const [dayKey, setDayKey] = useState(current?.toDateString() ?? days[0]?.key);
  const day = days.find((d) => d.key === dayKey) ?? days[0];
  const slots = useMemo(() => (day ? timeSlots(day.date, now) : []), [day, now]);
  const [picked, setPicked] = useState<string | undefined>(current?.toISOString());

  const done = (iso?: string) => {
    booking.setScheduledFor(iso);
    booking.setQuote(undefined); // re-price for the new time
    if (from === "home") router.replace("/book/route");
    else back();
  };

  return (
    <View style={[styles.root, { backgroundColor: palette.background, paddingTop: insets.top + 8 }]}>
      <View style={styles.pad}>
        <Header title="Schedule a ride" leading="close" onLeading={() => back()} />
        <Text variant="muted">
          Choose a pickup time from 30 minutes to 7 days ahead. We start finding your driver 15 minutes
          before.
        </Text>
      </View>
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        style={styles.dayRow}
        contentContainerStyle={styles.days}
      >
        {days.map((d) => (
          <ChoiceChip
            key={d.key}
            label={d.label}
            selected={d.key === day?.key}
            onPress={() => {
              setDayKey(d.key);
              setPicked(undefined);
            }}
          />
        ))}
      </ScrollView>
      <ScrollView
        style={styles.times}
        contentContainerStyle={[styles.grid, { paddingBottom: insets.bottom + 140 }]}
      >
        {slots.map((t, i) => (
          <Animated.View
            key={t.toISOString()}
            entering={FadeInDown.delay(Math.min(i, STAGGER_CAP) * STAGGER_MS).duration(220)}
            style={styles.cell}
          >
            <ChoiceChip
              label={clock(t)}
              selected={picked === t.toISOString()}
              onPress={() => setPicked(t.toISOString())}
              wide
            />
          </Animated.View>
        ))}
      </ScrollView>
      <View
        style={[
          styles.footer,
          {
            paddingBottom: insets.bottom + 16,
            backgroundColor: palette.background,
            borderTopColor: palette.border,
          },
        ]}
      >
        <PillButton
          title={picked ? `Set pickup time · ${pickupLabel(picked)}` : "Choose a time"}
          onPress={() => done(picked)}
          disabled={!picked}
        />
        {booking.scheduledFor ? (
          <Text
            style={[styles.now, { color: palette.primary }]}
            onPress={() => done(undefined)}
            accessibilityRole="button"
          >
            Ride now instead
          </Text>
        ) : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  pad: { paddingHorizontal: 20, gap: 8 },
  dayRow: { flexGrow: 0, flexShrink: 0, marginTop: 16 },
  days: { paddingHorizontal: 20, paddingVertical: 4, gap: 8, alignItems: "center" },
  times: { flex: 1 },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 8, padding: 20 },
  cell: { width: "31%" },
  footer: {
    position: "absolute",
    left: 0,
    right: 0,
    bottom: 0,
    paddingHorizontal: 20,
    paddingTop: 12,
    gap: 12,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  now: { textAlign: "center", fontSize: 16, fontWeight: "600", paddingVertical: 6 },
});
