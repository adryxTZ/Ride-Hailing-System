/**
 * ConsentScreen — "Your privacy": the current Privacy Notice's summary, links to it and the other texts it covers (Terms ...), and the
 * three choices (agree, read the full notice, decline and sign out). Shown by the
 * auth gate after sign-in, on start, or when a call answers CONSENT_REQUIRED.
 * Accepting clears the flag and the gate takes the person on; declining signs
 * out without storing anything.
 */
import { type ConsentDocument, isConsentUpdate } from "@platform/auth";
import { Button, Icon, ScalePressable, Screen, Text, useTheme, withAlpha } from "@platform/ui";
import { useState } from "react";
import { ActivityIndicator, Linking, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { useAcceptConsent, useConsentDocument } from "@/hooks/useConsent";
import { useProfile } from "@/hooks/useProfile";
import { auth } from "@/services/api";

const ENTER_MS = 260;
const ENTER_STAGGER_MS = 60;
const enter = (i: number) => FadeInDown.delay(i * ENTER_STAGGER_MS).duration(ENTER_MS);

/** The other documents this consent covers (Terms ...); the Privacy Notice has its own button. */
function otherLinks(doc?: ConsentDocument): ConsentDocument["links"] {
  return (doc?.links ?? []).filter((l) => l.kind !== "PRIVACY_NOTICE" && l.url);
}

export function ConsentScreen() {
  const { palette } = useTheme();
  const { data: doc, isLoading, error, refetch } = useConsentDocument();
  const { data: me } = useProfile();
  const accept = useAcceptConsent();
  const [declining, setDeclining] = useState(false);
  const updated = accept.stale || isConsentUpdate(me?.consent);
  const decline = async () => {
    setDeclining(true);
    await auth.logout();
  };
  return (
    <Screen>
      <Animated.View
        entering={enter(0)}
        style={[styles.badge, { backgroundColor: withAlpha(palette.primary, 0.12) }]}
      >
        <Icon name="shield-account-outline" size={34} color={palette.primary} />
      </Animated.View>
      <Animated.View entering={enter(1)} style={styles.block}>
        {updated ? (
          <Text style={{ color: palette.primary, fontWeight: "600" }}>
            We have updated our Privacy Notice.
          </Text>
        ) : null}
        <Text variant="title">Your privacy</Text>
      </Animated.View>
      <Animated.View
        entering={enter(2)}
        style={[styles.card, { backgroundColor: palette.surface, shadowColor: palette.text }]}
      >
        {isLoading ? <ActivityIndicator color={palette.primary} /> : null}
        {doc ? <Text>{doc.summary}</Text> : null}
        {error ? <Text variant="error">{error.message}</Text> : null}
        {error ? <Button title="Try again" variant="secondary" onPress={() => void refetch()} /> : null}
      </Animated.View>
      {accept.error && !accept.stale ? <Text variant="error">{accept.error.message}</Text> : null}
      <Animated.View entering={enter(3)} style={styles.block}>
        <Button
          title="I agree"
          loading={accept.isPending}
          disabled={!doc || declining}
          onPress={() => doc && accept.mutate(doc.version)}
        />
        <Button
          title="Read the Privacy Notice"
          variant="secondary"
          disabled={!doc}
          onPress={() => doc && void Linking.openURL(doc.url)}
        />
        {otherLinks(doc).map((l) => (
          <ScalePressable
            key={l.kind}
            accessibilityRole="link"
            accessibilityLabel={`Read the ${l.title}`}
            onPress={() => void Linking.openURL(l.url)}
            style={styles.link}
          >
            <Text style={[styles.linkText, { color: palette.primary }]}>{`Read the ${l.title}`}</Text>
          </ScalePressable>
        ))}
        <ScalePressable
          accessibilityRole="button"
          accessibilityLabel="Decline and sign out"
          onPress={() => void decline()}
          disabled={declining || accept.isPending}
          style={styles.decline}
        >
          <Text variant="muted" style={styles.declineText}>
            Decline and sign out
          </Text>
        </ScalePressable>
      </Animated.View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  badge: { width: 68, height: 68, borderRadius: 34, alignItems: "center", justifyContent: "center" },
  block: { gap: 12 },
  card: {
    borderRadius: 18,
    padding: 16,
    gap: 12,
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  link: { minHeight: 44, alignItems: "center", justifyContent: "center" },
  linkText: { fontWeight: "600" },
  decline: { minHeight: 44, alignItems: "center", justifyContent: "center" },
  declineText: { fontWeight: "600" },
});
