"use client";
/** Button — pill buttons: "primary" (black), "brand", "soft" (tinted), "ghost", "danger". */
import { Loader2 } from "lucide-react";

type Variant = "primary" | "brand" | "soft" | "ghost" | "danger";

const STYLE: Record<Variant, string> = {
  primary: "bg-nav text-nav-ink hover:opacity-90",
  brand: "bg-brand text-brand-ink hover:opacity-90",
  soft: "bg-surface-2 text-ink hover:bg-line",
  ghost: "text-ink hover:bg-surface-2",
  danger: "bg-critical text-white hover:opacity-90",
};

interface Props extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  loading?: boolean;
  icon?: React.ReactNode;
  size?: "sm" | "md";
}

export function Button({
  variant = "primary",
  loading,
  icon,
  size = "md",
  children,
  className = "",
  disabled,
  ...rest
}: Props) {
  const pad = size === "sm" ? "h-8 px-3 text-xs" : "h-10 px-4 text-sm";
  return (
    <button
      type="button"
      {...rest}
      disabled={disabled || loading}
      className={`inline-flex items-center justify-center gap-2 rounded-[8px] font-semibold transition-[opacity,background-color,transform] duration-150 active:scale-[.975] disabled:opacity-40 ${pad} ${STYLE[variant]} ${className}`}
    >
      {loading ? <Loader2 size={16} className="animate-spin" aria-hidden="true" /> : icon}
      {children}
    </button>
  );
}
