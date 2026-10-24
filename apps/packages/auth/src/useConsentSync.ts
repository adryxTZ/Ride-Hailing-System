/**
 * useConsentSync — keeps the consent flag in step with `GET /me`: whenever the
 * profile's `consent.required` changes it is copied into the store, and the
 * profile is refetched each time the app returns to the foreground (a new notice
 * may have been published while it was in the background).
 */
import { useEffect, useRef } from "react";
import { AppState } from "react-native";
import { useConsentStore } from "./consentStore";

const FOREGROUND = "active";

/**
 * @param required `Me.consent.required` from the app's profile query (undefined while loading).
 * @param refetch  refetches the profile.
 */
export function useConsentSync(required: boolean | undefined, refetch: () => unknown): void {
  const setRequired = useConsentStore((s) => s.setRequired);
  // A ref so a new refetch identity does not re-subscribe the listener.
  const refetchRef = useRef(refetch);
  refetchRef.current = refetch;

  useEffect(() => {
    if (required !== undefined) setRequired(required);
  }, [required, setRequired]);

  useEffect(() => {
    const sub = AppState.addEventListener("change", (state) => {
      if (state === FOREGROUND) refetchRef.current();
    });
    return () => sub.remove();
  }, []);
}
