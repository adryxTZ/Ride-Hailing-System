"use client";
/**
 * Portal — renders its children at the end of <body>. A dialog opened from
 * inside a Card needs it: the card's entrance animation (transform, filter)
 * makes a fixed-position backdrop cover only the card instead of the window.
 */
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";

export function Portal({ children }: { children: React.ReactNode }) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  return mounted ? createPortal(children, document.body) : null;
}
