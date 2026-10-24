/**
 * useAuthGate — applies authRoute: signed-out people go to sign-in, people who
 * owe a Privacy Notice acceptance go to the consent screen (so nobody goes online or submits a registration
 * before accepting), and everyone else leaves the auth and consent screens.
 * After accepting, the person returns to the screen the gate interrupted.
 */
import { useConsentRequired, useSession } from "@platform/auth";
import { useRouter, useSegments } from "expo-router";
import { useEffect } from "react";
import { useConsentWatch } from "@/hooks/useConsent";
import { authRoute } from "@/models/authRoute";

export function useAuthGate() {
  const { session, isHydrated } = useSession();
  const consentRequired = useConsentRequired();
  const segments = useSegments();
  const router = useRouter();
  useConsentWatch();
  const segment = segments[0] as string | undefined;
  useEffect(() => {
    if (!isHydrated) return;
    const move = authRoute(!!session, consentRequired, segment);
    if (move.kind === "sign-in") router.replace("/(auth)/phone");
    else if (move.kind === "consent" && move.stack) router.push("/consent");
    else if (move.kind === "consent") router.replace("/consent");
    else if (move.kind === "home") {
      if (move.fromConsent && router.canGoBack()) router.back();
      else router.replace("/");
    }
  }, [session, isHydrated, consentRequired, segment, router]);
}
