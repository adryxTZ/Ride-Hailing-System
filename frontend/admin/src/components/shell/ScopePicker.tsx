"use client";
/**
 * ScopePicker — the country and region every operations page looks at, as two
 * compact menus in the top bar. Choosing a country clears the region.
 */
import { ChevronDown, Globe2, MapPin } from "lucide-react";
import { useAreas, useCountries, useScope } from "@/lib/scope";
import { accessScopes } from "@/lib/session";

export function ScopePicker() {
  const { scope, set } = useScope();
  const { data: countries } = useCountries();
  const { data: areas } = useAreas(scope.country);
  const country = countries?.find((c) => c.code === scope.country);
  const term = country?.area_term?.en ?? "Region";
  // Limited staff see only their countries; a region-limited person only their regions.
  const access = accessScopes();
  const countryOk = (code: string) => !access.length || access.some((a) => a.country === code);
  const wholeCountry = !access.length || access.some((a) => a.country === scope.country && !a.areaId);
  const wholeCountryFor = (code: string) =>
    !access.length || access.some((a) => a.country === code && !a.areaId);
  const areaOk = (id: string) =>
    wholeCountry || access.some((a) => a.country === scope.country && a.areaId === id);
  return (
    <div className="flex w-full min-w-0 max-w-full items-center gap-1 rounded-[10px] bg-surface p-1 sm:w-auto">
      <PickerSelect
        label="Country"
        icon={<Globe2 size={15} aria-hidden="true" />}
        value={scope.country}
        onChange={(v) =>
          set({
            country: v,
            areaId: wholeCountryFor(v) ? undefined : access.find((a) => a.country === v)?.areaId,
          })
        }
        options={(countries ?? [])
          .filter((c) => countryOk(c.code))
          .map((c) => ({
            value: c.code,
            label: `${c.name?.en ?? c.code} · ${c.currency}${c.status === "DRAFT" ? " (draft)" : ""}`,
          }))}
      />
      <span className="h-5 w-px bg-line" aria-hidden="true" />
      <PickerSelect
        label={term}
        icon={<MapPin size={15} aria-hidden="true" />}
        value={scope.areaId ?? ""}
        onChange={(v) => set({ country: scope.country, areaId: v || undefined })}
        options={[
          ...(wholeCountry ? [{ value: "", label: `All ${term.toLowerCase()}s` }] : []),
          ...(areas ?? [])
            .filter((a) => areaOk(a.id))
            .map((a) => ({
              value: a.id,
              label: `${a.name}${a.status !== "ACTIVE" ? ` (${a.status.toLowerCase()})` : ""}`,
            })),
        ]}
      />
    </div>
  );
}

/** A native select dressed as a quiet button: icon, current value, chevron. */
function PickerSelect({
  label,
  icon,
  value,
  options,
  onChange,
}: {
  label: string;
  icon: React.ReactNode;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  const current = options.find((o) => o.value === value)?.label ?? label;
  return (
    <label className="relative flex h-9 min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-[8px] px-3 text-sm transition-colors hover:bg-surface-2 sm:flex-none">
      <span className="shrink-0 text-muted">{icon}</span>
      <span className="min-w-0 max-w-[180px] truncate font-medium">{current}</span>
      <ChevronDown size={14} aria-hidden="true" className="shrink-0 text-muted" />
      <select
        aria-label={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="absolute inset-0 cursor-pointer opacity-0"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}
