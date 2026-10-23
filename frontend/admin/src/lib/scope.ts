"use client";
/**
 * useScope — the country (and optional area) every operations page looks at.
 * Lives in the URL (?country=&area=) so links share it, and in localStorage so
 * it survives navigation; the brand's home country is the default.
 */
import { useQuery } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";
import { api, unwrap } from "@/lib/api";
import { useBrand } from "@/lib/brand";
import { accessScopes } from "@/lib/session";

const STORAGE_KEY = "admin.scope";

export interface Scope {
  country: string;
  areaId?: string;
}

/** A scope named in the page address (?country=KE&area=<id>), for shared links. */
function readUrl(): Scope | undefined {
  if (typeof window === "undefined") return undefined;
  const q = new URLSearchParams(window.location.search);
  const country = q.get("country")?.toUpperCase();
  return country ? { country, areaId: q.get("area") ?? undefined } : undefined;
}

function readStored(): Scope | undefined {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Scope) : undefined;
  } catch {
    return undefined;
  }
}

function remember(s: Scope) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
  } catch {
    // storage can be unavailable; the scope still works for this page
  }
}

export function useCountries() {
  return useQuery({
    queryKey: ["admin", "countries"],
    queryFn: async () => unwrap(await api.GET("/admin/countries")).data,
    staleTime: 60_000,
  });
}

export function useAreas(country?: string) {
  return useQuery({
    queryKey: ["admin", "areas", country],
    queryFn: async () => unwrap(await api.GET("/admin/areas", { params: { query: { country } } })).data,
    enabled: !!country,
    staleTime: 10_000,
  });
}

/** Whether a scope is inside the person's access (no limits = everything). */
export function allowed(s: Scope): boolean {
  const access = accessScopes();
  if (!access.length) return true;
  return access.some((a) => a.country === s.country && (!a.areaId || a.areaId === s.areaId));
}

/** The first scope a limited person may use (their first country or region). */
function firstAllowed(): Scope | undefined {
  const a = accessScopes()[0];
  return a ? { country: a.country, areaId: a.areaId } : undefined;
}

export function useScope() {
  const { data: brand } = useBrand();
  const [scope, setScopeState] = useState<Scope>(() => {
    const linked = readUrl();
    if (linked && allowed(linked)) {
      remember(linked);
      return linked;
    }
    const stored = readStored();
    return stored && allowed(stored) ? stored : (firstAllowed() ?? { country: "" });
  });
  useEffect(() => {
    if (!scope.country && brand?.brand.country)
      setScopeState(firstAllowed() ?? { country: brand.brand.country });
  }, [brand, scope.country]);
  const set = useCallback((next: Scope) => {
    setScopeState(next);
    remember(next);
    window.dispatchEvent(new Event("admin-scope"));
  }, []);
  // Other mounted pages follow a change made in the picker.
  useEffect(() => {
    const sync = () => {
      const s = readStored();
      if (s) setScopeState(s);
    };
    window.addEventListener("admin-scope", sync);
    return () => window.removeEventListener("admin-scope", sync);
  }, []);
  return { scope, set, query: { country: scope.country || undefined, area_id: scope.areaId } };
}

/**
 * The selected country's regulator, for labels: "LATRA levy" in Tanzania,
 * "Regulator levy" where the country has none configured. `rules` is true only
 * where the dashboard knows that regulator's limits (shown as field hints).
 */
export function useRegulator() {
  const { scope } = useScope();
  const { data: countries } = useCountries();
  const code = countries?.find((c) => c.code === scope.country)?.regulator ?? "";
  const name = code && code !== "none" ? code.toUpperCase() : "";
  return { name, levy: name ? `${name} levy` : "Regulator levy", rules: code === "latra" };
}
