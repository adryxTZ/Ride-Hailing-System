"use client";
/** useScopeLabel — "Tanzania · Dar es Salaam" for page subtitles, from the picker's scope. */
import { useAreas, useCountries, useScope } from "@/lib/scope";

export function useScopeLabel() {
  const { scope } = useScope();
  const { data: countries } = useCountries();
  const { data: areas } = useAreas(scope.country);
  const country = countries?.find((c) => c.code === scope.country);
  const area = areas?.find((a) => a.id === scope.areaId);
  const term = country?.area_term?.en ?? "Region";
  return {
    label: [country?.name?.en ?? scope.country, area?.name].filter(Boolean).join(" · "),
    areaName: area?.name,
    term,
    inArea: !!scope.areaId,
    currency: country?.currency ?? "",
  };
}
