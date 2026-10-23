/**
 * RatingForm — "How was your trip?": five stars, then chips that fit the score
 * (praise for 4–5, problems for 1–3) and an optional comment, then Submit.
 */
import { Icon, ScalePressable, Text, TextField, useTheme } from "@platform/ui";
import { useState } from "react";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import type { RatingAspect, RatingValue } from "./model";

const STARS = [1, 2, 3, 4, 5];
const GOOD_FROM = 4;

interface Props {
  title: string;
  praise: RatingAspect[];
  problems: RatingAspect[];
  submitting?: boolean;
  onSubmit: (v: RatingValue) => void;
  onSkip?: () => void;
}

export function RatingForm(p: Props) {
  const { palette } = useTheme();
  const [score, setScore] = useState(0);
  const [aspects, setAspects] = useState<string[]>([]);
  const [comment, setComment] = useState("");
  const chips = score >= GOOD_FROM ? p.praise : score > 0 ? p.problems : [];
  const pick = (n: number) => {
    if (n >= GOOD_FROM !== score >= GOOD_FROM) setAspects([]); // chips changed meaning
    setScore(n);
  };
  const toggle = (k: string) => setAspects((a) => (a.includes(k) ? a.filter((x) => x !== k) : [...a, k]));
  return (
    <View style={styles.wrap}>
      <Text style={styles.title}>{p.title}</Text>
      <View style={styles.stars}>
        {STARS.map((n) => (
          <ScalePressable key={n} accessibilityLabel={`${n} star${n > 1 ? "s" : ""}`} onPress={() => pick(n)}>
            <Icon
              name={n <= score ? "star" : "star-outline"}
              size={40}
              color={n <= score ? palette.primary : palette.muted}
            />
          </ScalePressable>
        ))}
      </View>
      {chips.length ? (
        <Animated.View entering={FadeInDown.duration(240)} style={styles.chips}>
          {chips.map((c) => {
            const on = aspects.includes(c.key);
            return (
              <ScalePressable
                key={c.key}
                onPress={() => toggle(c.key)}
                style={[
                  styles.chip,
                  {
                    backgroundColor: on ? palette.primary : palette.surface,
                    borderColor: on ? palette.primary : palette.border,
                  },
                ]}
              >
                <Text style={{ color: on ? (palette.on_primary ?? palette.background) : palette.text }}>
                  {c.label}
                </Text>
              </ScalePressable>
            );
          })}
        </Animated.View>
      ) : null}
      {score > 0 ? (
        <TextField
          placeholder="Add a comment (optional)"
          value={comment}
          onChangeText={setComment}
          maxLength={500}
        />
      ) : null}
      <ScalePressable
        disabled={score === 0 || p.submitting}
        onPress={() => p.onSubmit({ score, aspects, comment: comment.trim() || undefined })}
        style={[styles.submit, { backgroundColor: palette.primary, opacity: score === 0 ? 0.4 : 1 }]}
      >
        {p.submitting ? (
          <ActivityIndicator color={palette.on_primary ?? palette.background} />
        ) : (
          <Text style={[styles.submitText, { color: palette.on_primary ?? palette.background }]}>
            Submit rating
          </Text>
        )}
      </ScalePressable>
      {p.onSkip ? (
        <ScalePressable onPress={p.onSkip} style={styles.skip}>
          <Text variant="muted">Not now</Text>
        </ScalePressable>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 14 },
  title: { fontSize: 20, fontWeight: "700", textAlign: "center" },
  stars: { flexDirection: "row", justifyContent: "center", gap: 8 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8, justifyContent: "center" },
  chip: { borderWidth: 1, borderRadius: 18, paddingHorizontal: 14, height: 36, justifyContent: "center" },
  submit: { height: 56, borderRadius: 28, alignItems: "center", justifyContent: "center" },
  submitText: { fontSize: 17, fontWeight: "700" },
  skip: { alignSelf: "center", padding: 6 },
});
