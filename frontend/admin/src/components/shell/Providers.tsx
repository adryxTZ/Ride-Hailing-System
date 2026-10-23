"use client";
/** Providers — the query cache and the brand theme bridge for every page. */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { ThemeBridge } from "./ThemeBridge";

const STALE_MS = 15_000;

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { staleTime: STALE_MS, retry: 1, refetchOnWindowFocus: true } },
      }),
  );
  return (
    <QueryClientProvider client={client}>
      <ThemeBridge />
      {children}
    </QueryClientProvider>
  );
}
