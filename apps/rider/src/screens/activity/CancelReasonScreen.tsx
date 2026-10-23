/**
 * CancelReasonScreen — "What went wrong?" (design: Cancel reasons; video 1:13).
 * Single choice from the backend's reasons; the chosen row highlights; "Done"
 * sits at the bottom. On success the trip screen sees the trip end and moves on.
 */
import { Icon, RadioRow, Text, TextField, useTheme } from "@platform/ui";
import { useState } from "react";
import { Pressable, ScrollView, StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { PillButton } from "@/components/common/PillButton";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useBack } from "@/hooks/useBack";
import { useCancelReasons } from "@/hooks/useProfile";
import { useCancelTrip } from "@/hooks/useTrips";

const MIN_NOTE = 3;

export function CancelReasonScreen({ tripId }: { tripId?: string }) {
  const back = useBack();
  const insets = useSafeAreaInsets();
  const { palette } = useTheme();
  const { data: config } = useAppConfig();
  const { data: reasons } = useCancelReasons();
  const [selected, setSelected] = useState<string>();
  const [note, setNote] = useState("");
  const cancel = useCancelTrip();
  const locale = config?.brand.locale ?? "en";
  const list = reasons?.rider ?? [];
  const chosen = list.find((r) => r.key === selected);
  const needsNote = chosen?.free_text === true && note.trim().length < MIN_NOTE;

  const done = () => {
    if (!tripId || !selected) return back();
    cancel.mutate(
      { id: tripId, reason: selected, note: note.trim() || undefined },
      { onSuccess: () => back() },
    );
  };

  return (
    <View style={[styles.root, { backgroundColor: palette.background, paddingTop: insets.top + 12 }]}>
      <View style={styles.header}>
        <Pressable accessibilityLabel="Close" onPress={() => back()} hitSlop={10}>
          <Icon name="close" size={26} color={palette.text} />
        </Pressable>
        <Text style={styles.title}>What went wrong?</Text>
      </View>
      <ScrollView contentContainerStyle={styles.list} keyboardShouldPersistTaps="handled">
        {list.map((r) => (
          <View
            key={r.key}
            style={[
              styles.row,
              selected === r.key ? { backgroundColor: palette.surface } : null,
              { borderBottomColor: palette.border },
            ]}
          >
            <RadioRow
              label={r.label[locale] ?? r.label.en ?? r.key}
              selected={selected === r.key}
              onPress={() => setSelected(r.key)}
            />
          </View>
        ))}
        {chosen?.free_text ? (
          <TextField label="Tell us more" value={note} onChangeText={setNote} multiline />
        ) : null}
        {cancel.error ? <Text variant="error">{cancel.error.message}</Text> : null}
      </ScrollView>
      <View style={[styles.footer, { paddingBottom: insets.bottom + 16 }]}>
        <PillButton
          title="Done"
          loading={cancel.isPending}
          disabled={!selected || needsNote}
          onPress={done}
        />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  header: { flexDirection: "row", alignItems: "center", gap: 20, paddingHorizontal: 16, paddingBottom: 12 },
  title: { fontSize: 18, fontWeight: "600" },
  list: { paddingBottom: 24 },
  row: { paddingHorizontal: 16, borderBottomWidth: StyleSheet.hairlineWidth },
  footer: { paddingHorizontal: 16, paddingTop: 8 },
});
