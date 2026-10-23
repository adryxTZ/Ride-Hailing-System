/** brand — the brand's name and theme from GET /v1/app/config (cached for the session). */
import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";

const CONFIG_STALE_MS = 5 * 60_000;

export function useBrand() {
  return useQuery({
    queryKey: ["app-config"],
    queryFn: async () => unwrap(await api.GET("/app/config")),
    staleTime: CONFIG_STALE_MS,
  });
}
