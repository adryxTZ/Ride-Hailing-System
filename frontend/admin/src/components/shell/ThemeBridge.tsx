"use client";
/** ThemeBridge — puts the brand's primary colour and name into the page (no colour literals in components). */
import { useEffect } from "react";
import { useBrand } from "@/lib/brand";

export function ThemeBridge() {
  const { data } = useBrand();
  useEffect(() => {
    const p = data?.theme?.palette as { primary?: string; on_primary?: string } | undefined;
    const root = document.documentElement;
    if (p?.primary) root.style.setProperty("--brand", p.primary);
    if (p?.on_primary) root.style.setProperty("--brand-ink", p.on_primary);
    if (data?.brand.name) document.title = `${data.brand.name} · Operations`;
  }, [data]);
  return null;
}
