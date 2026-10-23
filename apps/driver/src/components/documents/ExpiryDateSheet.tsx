/**
 * ExpiryDateSheet — asks for a document's expiry date after the photo is
 * taken: required for some documents (no skip), optional for others.
 */
import { Button, Icon, Text, TextField, useTheme, withAlpha } from "@platform/ui";
import { useState } from "react";
import { Modal, Pressable, StyleSheet } from "react-native";
import Animated, { Easing, FadeIn, FadeOut, SlideInDown, SlideOutDown } from "react-native-reanimated";
import { expiryProblem } from "@/models/document";

const ENTER_MS = 280;
const EXIT_MS = 240;
const ease = Easing.out(Easing.cubic);

interface Props {
  label: string;
  required: boolean;
  bottom: number;
  onDone: (date?: string) => void;
  onCancel: () => void;
}

export function ExpiryDateSheet({ label, required, bottom, onDone, onCancel }: Props) {
  const { palette } = useTheme();
  const [value, setValue] = useState("");
  const problem = value ? expiryProblem(value) : "";
  return (
    <Modal transparent visible animationType="none" onRequestClose={onCancel}>
      <Animated.View
        entering={FadeIn.duration(ENTER_MS).easing(ease)}
        exiting={FadeOut.duration(EXIT_MS).easing(ease)}
        style={[StyleSheet.absoluteFill, styles.scrim, { backgroundColor: withAlpha(palette.text, 0.35) }]}
      >
        <Pressable style={StyleSheet.absoluteFill} onPress={onCancel} accessibilityLabel="Close" />
        <Animated.View
          entering={SlideInDown.duration(ENTER_MS).easing(ease)}
          exiting={SlideOutDown.duration(EXIT_MS).easing(ease)}
          style={[styles.sheet, { backgroundColor: palette.background, paddingBottom: bottom + 20 }]}
        >
          <Pressable
            onPress={onCancel}
            hitSlop={8}
            style={[styles.close, { backgroundColor: palette.surface }]}
            accessibilityLabel="Close"
            accessibilityRole="button"
          >
            <Icon name="close" size={22} color={palette.text} />
          </Pressable>
          <Text style={styles.title}>{`When does your ${label.toLowerCase()} expire?`}</Text>
          <Text variant="muted">We remind you before it expires so you can keep driving.</Text>
          <TextField
            label="Expiry date"
            value={value}
            onChangeText={setValue}
            placeholder="YYYY-MM-DD"
            keyboardType="numbers-and-punctuation"
            autoFocus
            error={problem || undefined}
          />
          <Button title="Save" onPress={() => onDone(value)} disabled={!value || !!problem} />
          {!required ? (
            <Text onPress={() => onDone(undefined)} style={[styles.link, { color: palette.primary }]}>
              It has no expiry date
            </Text>
          ) : null}
        </Animated.View>
      </Animated.View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  scrim: { justifyContent: "flex-end" },
  sheet: { borderTopLeftRadius: 24, borderTopRightRadius: 24, padding: 20, gap: 14 },
  close: {
    position: "absolute",
    top: 12,
    right: 12,
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: "center",
    justifyContent: "center",
  },
  title: { fontSize: 22, fontWeight: "700", marginTop: 8, marginRight: 52 },
  link: { fontSize: 16, fontWeight: "600", paddingVertical: 8, textAlign: "center" },
});
