/**
 * useBack — "back" that always lands somewhere: the previous screen when there
 * is one, otherwise the fallback (a screen opened directly, from a link or a
 * web refresh, has no history and router.back() would do nothing).
 */
import { type Href, useRouter } from "expo-router";
import { useCallback } from "react";

/** Where back goes when there is no history: the app's home. */
const HOME: Href = "/";

export function useBack(fallback: Href = HOME) {
  const router = useRouter();
  return useCallback(() => {
    if (router.canGoBack()) router.back();
    else router.replace(fallback);
  }, [router, fallback]);
}
