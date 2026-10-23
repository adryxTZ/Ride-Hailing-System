/** SafetyScreen — trusted contacts (up to 3) who get a live link when the rider presses SOS. */
import { Button, Header, Icon, ScalePressable, Screen, Text, TextField, useTheme } from "@platform/ui";
import { useEffect, useState } from "react";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown, LinearTransition } from "react-native-reanimated";
import { useBack } from "@/hooks/useBack";
import { useSaveTrustedContacts, useTrustedContacts } from "@/hooks/useSafety";
import type { TrustedContact } from "@/services/safety";

const MAX_CONTACTS = 3;

export function SafetyScreen() {
  const back = useBack();
  const { palette } = useTheme();
  const { data } = useTrustedContacts();
  const save = useSaveTrustedContacts();
  const [list, setList] = useState<TrustedContact[]>([]);
  useEffect(() => {
    if (data) setList(data);
  }, [data]);
  const update = (i: number, patch: Partial<TrustedContact>) =>
    setList((l) => l.map((c, j) => (j === i ? { ...c, ...patch } : c)));
  const ready = list.every((c) => c.name.trim() && c.phone.replace(/\D/g, "").length >= 9);

  return (
    <Screen align="top">
      <Header leading="back" onLeading={() => back()} title="Safety" large />
      <Text variant="muted">
        Trusted contacts get a text with a live link to your trip when you press SOS during a ride. You can
        also share any trip from the Safety button on the map.
      </Text>
      {list.map((c, i) => (
        <Animated.View
          key={`contact-${i}`}
          entering={FadeInDown.delay(i * 60).duration(240)}
          layout={LinearTransition}
          style={[styles.card, { backgroundColor: palette.surface, borderColor: palette.border }]}
        >
          <View style={styles.head}>
            <Text style={styles.label}>{`Contact ${i + 1}`}</Text>
            <ScalePressable
              accessibilityLabel={`Remove contact ${i + 1}`}
              onPress={() => setList((l) => l.filter((_, j) => j !== i))}
              style={styles.remove}
            >
              <Icon name="close" size={20} color={palette.text} />
            </ScalePressable>
          </View>
          <TextField
            placeholder="Name"
            value={c.name}
            onChangeText={(name) => update(i, { name })}
            autoCapitalize="words"
          />
          <TextField
            placeholder="07XX XXX XXX"
            value={c.phone}
            onChangeText={(phone) => update(i, { phone })}
            keyboardType="phone-pad"
          />
        </Animated.View>
      ))}
      {list.length < MAX_CONTACTS ? (
        <Button
          title="Add a trusted contact"
          variant="secondary"
          onPress={() => setList((l) => [...l, { name: "", phone: "" }])}
        />
      ) : null}
      {save.error ? <Text variant="error">{save.error.message}</Text> : null}
      {save.isSuccess ? <Text style={{ color: palette.primary }}>Saved</Text> : null}
      <Button title="Save" onPress={() => save.mutate(list)} loading={save.isPending} disabled={!ready} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  card: { borderWidth: 1, borderRadius: 18, padding: 14, gap: 10 },
  head: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  label: { fontWeight: "700" },
  remove: { width: 44, height: 44, alignItems: "center", justifyContent: "center" },
});
