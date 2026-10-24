/** Root layout: providers + the auth gate. Screens live in src/screens. */
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { SplashReveal } from "@/components/common/SplashReveal";
import { useActiveTripRedirect } from "@/hooks/useActiveTripRedirect";
import { useAuthGate } from "@/hooks/useAuthGate";
import { AppProviders } from "@/providers/AppProviders";

export default function RootLayout() {
  return (
    <AppProviders>
      <StatusBar style="auto" />
      <AuthGate />
      <SplashReveal />
    </AppProviders>
  );
}

/**
 * Sends signed-out users to the auth group, people who must accept the Privacy
 * Notice to the consent screen, and everyone else out of both (useAuthGate).
 */
function AuthGate() {
  useAuthGate();
  useActiveTripRedirect();
  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Screen name="(tabs)" />
      {/* No swipe back: the gate would only bring the person straight back here. */}
      <Stack.Screen name="consent" options={{ gestureEnabled: false, animation: "fade" }} />
      <Stack.Screen
        name="cancel-reason"
        options={{ presentation: "modal", animation: "slide_from_bottom" }}
      />
      <Stack.Screen name="book/route" options={{ animation: "fade_from_bottom" }} />
      <Stack.Screen name="book/options" options={{ animation: "fade" }} />
      <Stack.Screen name="book/requested" options={{ animation: "fade" }} />
      <Stack.Screen name="book/pick" options={{ animation: "slide_from_right" }} />
      <Stack.Screen
        name="book/schedule"
        options={{ presentation: "modal", animation: "slide_from_bottom" }}
      />
      <Stack.Screen name="book/scheduled" options={{ animation: "fade" }} />
    </Stack>
  );
}
