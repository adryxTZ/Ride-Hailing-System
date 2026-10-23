import { useEffect } from "react";
import { useSessionStore } from "./session";

/** Loads the persisted session on first use and exposes it. `session` is undefined until hydrated. */
export function useSession() {
  const session = useSessionStore((s) => s.session);
  const hydrate = useSessionStore((s) => s.hydrate);
  useEffect(() => {
    if (session === undefined) void hydrate();
  }, [session, hydrate]);
  return { session, isHydrated: session !== undefined, isSignedIn: !!session };
}
