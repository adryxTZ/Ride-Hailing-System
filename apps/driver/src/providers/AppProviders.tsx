/** AppProviders — query cache, server theme and session hydration. Nothing else lives here. */
import { useSession } from "@platform/auth";
import { ThemeProvider } from "@platform/ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useDriverRealtime } from "@/hooks/useDriverTrips";
import { usePush } from "@/hooks/usePush";

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
  useDriverRealtime(); // one socket while signed in: offers and trip updates on any tab
  return <ThemeProvider theme={data?.theme}>{children}</ThemeProvider>;
}
