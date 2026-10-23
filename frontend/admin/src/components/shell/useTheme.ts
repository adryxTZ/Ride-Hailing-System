"use client";
/** useTheme — light/dark toggle stored per browser (the system setting until chosen). */
import { useEffect, useState } from "react";

const KEY = "platform.admin.theme";

export function useTheme() {
  const [dark, setDark] = useState(false);
  useEffect(() => {
    let saved: string | null = null;
    try {
      saved = window.localStorage.getItem(KEY);
    } catch {}
    const isDark = saved ? saved === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
    setDark(isDark);
    if (saved) document.documentElement.dataset.theme = saved;
  }, []);
  const toggle = () => {
    const next = dark ? "light" : "dark";
    setDark(!dark);
    document.documentElement.dataset.theme = next;
    try {
      window.localStorage.setItem(KEY, next);
    } catch {}
  };
  return { dark, toggle };
}
