/** AppProviders — query cache, server theme, session hydration and the realtime socket. */
import { useSession } from "@platform/auth";
import { ThemeProvider } from "@platform/ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { useAppConfig } from "@/hooks/useAppConfig";
import { usePush } from "@/hooks/usePush";
import { useTripRealtime } from "@/hooks/useTripRealtime";

const queryClient = new QueryClient();

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemedTree>{children}</ThemedTree>
    </QueryClientProvider>
  );
}

function ThemedTree({ children }: PropsWithChildren) {
  const { data } = useAppConfig();
  useSession(); // hydrate persisted session as early as possible
  usePush(); // push token registration + open-on-tap
  useTripRealtime(); // one socket while signed in; keeps trip caches live
  return <ThemeProvider theme={data?.theme}>{children}</ThemeProvider>;
}
