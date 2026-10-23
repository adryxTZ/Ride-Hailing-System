/** useProfile / useUpdateProfile — React Query hooks over the driver's own profile. */
import { useSession } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchMe, updateMe } from "@/services/profile";

const PROFILE_STALE_MS = 60_000;

export function useProfile() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["me"],
    queryFn: ({ signal }) => fetchMe(signal),
    staleTime: PROFILE_STALE_MS,
    enabled: isSignedIn,
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({ mutationFn: updateMe, onSuccess: (me) => qc.setQueryData(["me"], me) });
}
