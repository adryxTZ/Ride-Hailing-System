"use client";
/**
 * UserMenu — the signed-in person at the top right: photo (or initials), name
 * and role; opens a menu with their details, theme, staff tools and sign out.
 * Closes on outside click and Escape; keyboard reachable.
 */
import { ChevronDown, FileClock, LogOut, Moon, Sun, UserCog } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { EASE } from "@/components/ui/motion";
import { useMe } from "@/features/me";
import { clearSession } from "@/lib/session";
import { useTheme } from "./useTheme";

export function UserMenu() {
  const me = useMe();
  const router = useRouter();
  const { dark, toggle } = useTheme();
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const go = (href: string) => {
    setOpen(false);
    router.push(href);
  };
  const role = me.roles.join(", ") || "No role";

  return (
    <div ref={box} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2.5 rounded-[10px] bg-surface py-1.5 pl-1.5 pr-3 transition-colors hover:bg-surface-2"
      >
        <Avatar name={me.name} photoUrl={me.photoUrl} />
        <span className="hidden text-left leading-tight md:block">
          <span className="block max-w-[160px] truncate text-sm font-semibold">{me.name ?? "Staff"}</span>
          <span className="block max-w-[160px] truncate text-xs text-muted">{role}</span>
        </span>
        <ChevronDown
          size={15}
          aria-hidden="true"
          className={`text-muted transition-transform duration-200 ${open ? "rotate-180" : ""}`}
        />
      </button>
      <AnimatePresence>
        {open ? (
          <motion.div
            role="menu"
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.18, ease: EASE }}
            className="absolute right-0 top-[calc(100%+8px)] z-50 w-72 overflow-hidden rounded-[10px] bg-surface shadow-[0_12px_32px_rgba(0,0,0,0.12)] ring-1 ring-line"
          >
            <div className="flex items-center gap-3 border-b border-line p-4">
              <Avatar name={me.name} photoUrl={me.photoUrl} size="lg" />
              <div className="min-w-0">
                <p className="truncate font-semibold">{me.name ?? "Staff"}</p>
                {me.email ? <p className="truncate text-xs text-muted">{me.email}</p> : null}
                {me.phone ? <p className="tnum truncate text-xs text-muted">{me.phone}</p> : null}
                <p className="mt-1 truncate text-xs text-brand-text">{role}</p>
              </div>
            </div>
            <div className="grid p-1.5">
              <MenuItem
                icon={dark ? <Sun size={16} /> : <Moon size={16} />}
                label={dark ? "Light theme" : "Dark theme"}
                onClick={toggle}
              />
              {me.can("staff.manage") ? (
                <MenuItem icon={<UserCog size={16} />} label="Staff and roles" onClick={() => go("/staff")} />
              ) : null}
              <MenuItem icon={<FileClock size={16} />} label="Audit log" onClick={() => go("/audit")} />
            </div>
            <div className="border-t border-line p-1.5">
              <MenuItem
                icon={<LogOut size={16} />}
                label="Sign out"
                danger
                onClick={() => {
                  clearSession();
                  router.replace("/login");
                }}
              />
            </div>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </div>
  );
}

function MenuItem({
  icon,
  label,
  onClick,
  danger,
}: {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      onClick={onClick}
      className={`flex w-full items-center gap-3 rounded-[8px] px-3 py-2 text-left text-sm transition-colors hover:bg-surface-2 ${danger ? "text-critical" : ""}`}
    >
      <span aria-hidden="true" className={danger ? "" : "text-muted"}>
        {icon}
      </span>
      {label}
    </button>
  );
}

/** The person's photo, or their initials on the brand tint. */
function Avatar({ name, photoUrl, size = "md" }: { name?: string; photoUrl?: string; size?: "md" | "lg" }) {
  const box = size === "lg" ? "size-11 text-base" : "size-9 text-sm";
  const initials = (name ?? "?")
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join("");
  if (photoUrl) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={photoUrl} alt="" className={`${box} shrink-0 rounded-full object-cover`} />;
  }
  return (
    <span
      className={`${box} grid shrink-0 place-items-center rounded-full bg-brand-soft font-semibold text-brand-text`}
      aria-hidden="true"
    >
      {initials || "?"}
    </span>
  );
}
