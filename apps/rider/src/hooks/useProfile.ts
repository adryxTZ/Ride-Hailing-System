/** useProfile / usePlaces — React Query hooks over the profile service, with mutations that refresh the cache. */
import { useSession } from "@platform/auth";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  deletePlace,
  fetchCancelReasons,
  fetchMe,
  fetchPlaces,
  type PlaceInput,
  savePlace,
  updateMe,
} from "@/services/profile";

export function useProfile() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["me"],
    queryFn: ({ signal }) => fetchMe(signal),
    staleTime: 60_000,
    enabled: isSignedIn,
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({ mutationFn: updateMe, onSuccess: (me) => qc.setQueryData(["me"], me) });
}

export function usePlaces() {
  const { isSignedIn } = useSession();
  return useQuery({
    queryKey: ["places"],
    queryFn: ({ signal }) => fetchPlaces(signal),
    staleTime: 60_000,
    enabled: isSignedIn,
  });
}

export function useSavePlace() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: PlaceInput) => savePlace(p),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["places"] }),
  });
}

export function useDeletePlace() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deletePlace,
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["places"] }),
  });
}

export function useCancelReasons() {
  return useQuery({
    queryKey: ["cancel-reasons"],
    queryFn: ({ signal }) => fetchCancelReasons(signal),
    staleTime: 60 * 60_000,
  });
}
