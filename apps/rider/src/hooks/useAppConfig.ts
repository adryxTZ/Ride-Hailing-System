/** useAppConfig — React Query hook over the appConfig service; one-minute cache. */
import { useQuery } from "@tanstack/react-query";
import { fetchAppConfig } from "@/services/appConfig";

export function useAppConfig() {
  return useQuery({
    queryKey: ["app-config"],
    queryFn: ({ signal }) => fetchAppConfig(signal),
    staleTime: 60_000,
  });
}
