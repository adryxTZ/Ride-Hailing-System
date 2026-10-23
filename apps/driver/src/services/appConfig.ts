/** appConfig service — GET /v1/app/config through the shared client. Pure data access. */
import { api } from "@/services/api";

export async function fetchAppConfig(signal?: AbortSignal) {
  const { data, error } = await api.GET("/app/config", { signal });
  if (error || !data) throw new Error("app/config failed");
  return data;
}

export type AppConfig = Awaited<ReturnType<typeof fetchAppConfig>>;
