/** useServices — cached catalogue (ETag on the wire, five minutes in memory). */
import { useQuery } from "@tanstack/react-query";
import { fetchServices } from "@/services/catalog";

export function useServices() {
  return useQuery({
    queryKey: ["services"],
    queryFn: ({ signal }) => fetchServices(signal),
    staleTime: 5 * 60_000,
  });
}

/** One service by key from the cached list. */
export function useService(key: string) {
  const q = useServices();
  return { ...q, service: q.data?.find((s) => s.key === key) };
}
