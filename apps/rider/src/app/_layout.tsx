/** Root layout: providers + the auth gate. Screens live in src/screens. */
import { useSession } from "@platform/auth";
import { Stack, useRouter, useSegments } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useEffect } from "react";
import { SplashReveal } from "@/components/common/SplashReveal";
import { useActiveTripRedirect } from "@/hooks/useActiveTripRedirect";
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

/** Sends signed-out users to the auth group and signed-in users out of it. */
function AuthGate() {
  const { session, isHydrated } = useSession();
  const segments = useSegments();
  const router = useRouter();
  useActiveTripRedirect();
  useEffect(() => {
    if (!isHydrated) return;
    const inAuthGroup = segments[0] === "(auth)";
    if (!session && !inAuthGroup) router.replace("/(auth)/phone");
    if (session && inAuthGroup) router.replace("/");
  }, [session, isHydrated, segments, router]);
  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Screen name="(tabs)" />
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
