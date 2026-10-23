/** useTaxiClasses — the taxi service's active vehicle classes (Bajaji, Standard, XL …), cached. */
import { useQuery } from "@tanstack/react-query";
import { SERVICE_KEY } from "@/config";
import { api } from "@/services/api";

const CATALOGUE_STALE_MS = 5 * 60_000;

export function useTaxiClasses() {
  const q = useQuery({
    queryKey: ["services"],
    queryFn: async ({ signal }) => {
      const { data } = await api.GET("/services", { signal });
      return data?.data ?? [];
    },
    staleTime: CATALOGUE_STALE_MS,
  });
  const service = q.data?.find((s) => s.key === SERVICE_KEY);
  return { ...q, classes: (service?.vehicle_classes ?? []).filter((c) => c.status === "ACTIVE") };
}

type LocalText = Record<string, string> | undefined;

/** Text for the locale, falling back to English then any value. */
export const localText = (t: LocalText, locale: string) =>
  t ? (t[locale] ?? t.en ?? Object.values(t)[0] ?? "") : "";
