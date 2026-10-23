"use client";
/** useMe — the signed-in staff member: name from /me, roles from the token. */
import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";
import { permissions, staffRoles } from "@/lib/session";

export function useMe() {
  const q = useQuery({ queryKey: ["me"], queryFn: async () => unwrap(await api.GET("/me")) });
  const roles = staffRoles();
  const perms = permissions();
  return {
    name: q.data?.full_name ?? q.data?.phone,
    email: q.data?.email ?? undefined,
    phone: q.data?.phone,
    photoUrl: q.data?.photo_url ?? undefined,
    roles,
    /** True when the person holds one of the permissions ("*" holds them all). */
    can: (...want: string[]) => perms.includes("*") || want.some((p) => perms.includes(p)),
  };
}
