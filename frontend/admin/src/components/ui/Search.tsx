"use client";
/** Search — a rounded search box that reports after a short pause in typing. */
import { Search as Icon, X } from "lucide-react";
import { useEffect, useState } from "react";

const DEBOUNCE_MS = 300;

export function Search({
  value,
  onChange,
  placeholder,
  inset = false,
}: {
  value?: string;
  onChange: (v: string) => void;
  placeholder: string;
  /** Inside a card: sits on the card's inset tone instead of the page's surface. */
  inset?: boolean;
}) {
  const [text, setText] = useState(value ?? "");
  useEffect(() => setText(value ?? ""), [value]);
  useEffect(() => {
    const t = setTimeout(() => {
      if (text !== (value ?? "")) onChange(text);
    }, DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [text, value, onChange]);
  return (
    <label
      className={`flex h-11 min-w-0 items-center gap-2 rounded-[8px] px-3 ${inset ? "bg-surface-2" : "bg-surface"}`}
    >
      <Icon size={16} className="text-muted" aria-hidden="true" />
      <input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className={`min-w-0 ${inset ? "flex-1" : "w-full sm:w-64"} bg-transparent text-sm outline-none placeholder:text-muted`}
      />
      {text ? (
        <button
          type="button"
          aria-label="Clear search"
          onClick={() => setText("")}
          className="grid size-6 shrink-0 place-items-center rounded text-muted hover:bg-line hover:text-ink"
        >
          <X size={13} aria-hidden="true" />
        </button>
      ) : null}
    </label>
  );
}
