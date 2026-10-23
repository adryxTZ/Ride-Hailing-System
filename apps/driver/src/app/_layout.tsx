/** Root layout: providers + the auth gate. Screens live in src/screens. */
import { useSession } from "@platform/auth";
import { Stack, useRouter, useSegments } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useEffect } from "react";
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

/** Sends signed-out users to the auth group and signed-in users out of it. */
function AuthGate() {
  const { session, isHydrated } = useSession();
  const segments = useSegments();
  const router = useRouter();
  useWorkRedirect();
  useEffect(() => {
    if (!isHydrated) return;
    const inAuthGroup = segments[0] === "(auth)";
    if (!session && !inAuthGroup) router.replace("/(auth)/phone");
    if (session && inAuthGroup) router.replace("/");
  }, [session, isHydrated, segments, router]);
  return <Stack screenOptions={{ headerShown: false }} />;
}
