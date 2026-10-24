/** Root layout: providers + the auth gate. Screens live in src/screens. */
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useAuthGate } from "@/hooks/useAuthGate";
import { useWorkRedirect } from "@/hooks/useWorkRedirect";
import { AppProviders } from "@/providers/AppProviders";

export default function RootLayout() {
  return (
    <AppProviders>
      <StatusBar style="auto" />
      <AuthGate />
    </AppProviders>
  );
}

/**
 * Sends signed-out users to the auth group, people who must accept the Privacy
 * Notice to the consent screen, and everyone else out of both (useAuthGate).
 */
function AuthGate() {
  useAuthGate();
  useWorkRedirect();
  return (
    <Stack screenOptions={{ headerShown: false }}>
      {/* No swipe back: the gate would only bring the person straight back here. */}
      <Stack.Screen name="consent" options={{ gestureEnabled: false, animation: "fade" }} />
    </Stack>
  );
}
