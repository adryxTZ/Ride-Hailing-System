"use client";
/**
 * Sidebar — the dashboard's side navigation: brand at the top, Operate pages
 * always shown, and the other groups as dropdowns, one open at a time. The
 * group holding the current page opens by itself; the active
 * page is a dark pill that glides between items. Labels hide on narrow screens.
 */
import { ChevronDown } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { EASE } from "@/components/ui/motion";
import { useApprovalCount } from "@/features/approvals/hooks";
import { useOpenSos } from "@/features/safety/hooks";
import { useBrand } from "@/lib/brand";
import { isActive, NAV_GROUPS, type NavGroup, type NavItem } from "./nav";

const OPEN_KEY = "admin.nav.open-group";

/** A count per page href, shown as a badge on its link. */
type Badges = Record<string, number>;

export function Sidebar() {
  const pathname = usePathname();
  const { data: brand } = useBrand();
  const sos = useOpenSos();
  const approvals = useApprovalCount().data ?? 0;
  // Counts shown next to a page: open SOS alerts, approvals waiting for this person.
  const badges: Badges = { "/safety": sos, "/approvals": approvals };
  const name = brand?.brand.name ?? "";
  const [open, setOpen] = useState<string>();

  // One group open at a time (an accordion): the group of the current page opens
  // by itself; otherwise the last group the person opened is remembered.
  useEffect(() => {
    const current = NAV_GROUPS.find((g) => g.collapsible && g.items.some((i) => isActive(pathname, i.href)));
    if (current) {
      setOpen(current.title);
      return;
    }
    try {
      setOpen(localStorage.getItem(OPEN_KEY) ?? undefined);
    } catch {
      // no storage: start closed
    }
  }, [pathname]);
  const toggle = (title: string) =>
    setOpen((o) => {
      const next = o === title ? undefined : title;
      try {
        if (next) localStorage.setItem(OPEN_KEY, next);
        else localStorage.removeItem(OPEN_KEY);
      } catch {
        // storage unavailable; the state still works for this visit
      }
      return next;
    });

  return (
    <aside className="sticky top-0 flex h-dvh w-16 shrink-0 flex-col gap-6 bg-surface px-3 py-5 lg:w-60 lg:px-4">
      <Link href="/" className="flex items-center gap-2.5 px-1" aria-label={`${name} home`}>
        <span className="grid size-9 shrink-0 place-items-center rounded-full bg-brand text-sm font-bold text-brand-ink">
          {name.slice(0, 1)}
        </span>
        <span className="hidden truncate font-semibold lg:block">{name}</span>
      </Link>
      <nav aria-label="Sections" className="no-scrollbar flex flex-1 flex-col gap-4 overflow-y-auto">
        {NAV_GROUPS.map((group) =>
          group.collapsible ? (
            <DropdownGroup
              key={group.title}
              group={group}
              pathname={pathname}
              open={open === group.title}
              onToggle={() => toggle(group.title)}
              badges={badges}
            />
          ) : (
            <div key={group.title} className="grid gap-1">
              <p className="mb-1 hidden px-3 text-[11px] font-semibold uppercase tracking-wide text-muted lg:block">
                {group.title}
              </p>
              {group.items.map((item) => (
                <NavLink
                  key={item.href}
                  item={item}
                  active={isActive(pathname, item.href)}
                  badge={badges[item.href] ?? 0}
                />
              ))}
            </div>
          ),
        )}
      </nav>
    </aside>
  );
}

/** A group shown as one button; opening it reveals its pages, indented. */
function DropdownGroup({
  group,
  pathname,
  open,
  onToggle,
  badges,
}: {
  group: NavGroup;
  pathname: string;
  open: boolean;
  onToggle: () => void;
  badges: Badges;
}) {
  const Icon = group.icon;
  const holdsActive = group.items.some((i) => isActive(pathname, i.href));
  const badge = group.items.reduce((n, i) => n + (badges[i.href] ?? 0), 0);
  const id = `nav-${group.title.toLowerCase()}`;
  return (
    <div className="grid gap-1">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={id}
        title={group.title}
        onClick={onToggle}
        className={`flex items-center gap-3 rounded-[10px] px-3 py-2.5 text-sm transition-colors duration-200 hover:bg-surface-2 ${holdsActive ? "font-semibold text-ink" : "text-muted hover:text-ink"}`}
      >
        {Icon ? <Icon size={18} aria-hidden="true" className="shrink-0" /> : null}
        <span className="hidden flex-1 truncate text-left lg:block">{group.title}</span>
        {badge && !open ? (
          <span className="tnum grid min-w-5 place-items-center rounded-full bg-critical px-1.5 text-[11px] font-bold text-white">
            {badge}
          </span>
        ) : null}
        <ChevronDown
          size={15}
          aria-hidden="true"
          className={`hidden shrink-0 transition-transform duration-200 lg:block ${open ? "rotate-180" : ""}`}
        />
      </button>
      <AnimatePresence initial={false}>
        {open ? (
          <motion.div
            id={id}
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.24, ease: EASE }}
            className="grid gap-1 overflow-hidden lg:ml-3 lg:border-l lg:border-line lg:pl-2"
          >
            {group.items.map((item) => (
              <NavLink
                key={item.href}
                item={item}
                active={isActive(pathname, item.href)}
                badge={badges[item.href] ?? 0}
              />
            ))}
          </motion.div>
        ) : null}
      </AnimatePresence>
    </div>
  );
}

function NavLink({ item, active, badge }: { item: NavItem; active: boolean; badge: number }) {
  const Icon = item.icon;
  return (
    <Link
      href={item.href}
      title={item.label}
      aria-current={active ? "page" : undefined}
      className={`relative flex items-center gap-3 rounded-[10px] px-3 py-2.5 text-sm transition-colors duration-200 ${active ? "text-nav-ink" : "text-muted hover:text-ink"}`}
    >
      {active ? (
        <motion.span
          layoutId="side-nav-pill"
          className="absolute inset-0 rounded-[10px] bg-nav"
          transition={{ duration: 0.35, ease: [0.32, 0.72, 0, 1] }}
        />
      ) : null}
      <Icon size={18} aria-hidden="true" className="relative shrink-0" />
      <span className="relative hidden flex-1 truncate lg:block">{item.label}</span>
      {badge ? (
        <span className="tnum relative grid min-w-5 place-items-center rounded-full bg-critical px-1.5 text-[11px] font-bold text-white">
          {badge}
        </span>
      ) : null}
    </Link>
  );
}
