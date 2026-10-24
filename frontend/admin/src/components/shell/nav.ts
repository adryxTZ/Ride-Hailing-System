/** The dashboard's sections, grouped for the side navigation. */
import {
  Car,
  Cpu,
  FileCheck,
  FileClock,
  Hexagon,
  KeyRound,
  Landmark,
  LayoutGrid,
  LockKeyhole,
  Map as MapIcon,
  MapPinned,
  Palette,
  Percent,
  Receipt,
  Route,
  ScanSearch,
  Settings,
  ShieldAlert,
  ShieldCheck,
  Tag,
  UserCog,
  Users,
  Wrench,
} from "lucide-react";

export interface NavItem {
  href: string;
  label: string;
  icon: typeof LayoutGrid;
}

export interface NavGroup {
  title: string;
  /** Shown as a single button that opens to reveal its items (Pricing, Money, Tools). */
  collapsible?: boolean;
  icon?: typeof LayoutGrid;
  items: NavItem[];
}

export const NAV_GROUPS: NavGroup[] = [
  {
    title: "Operate",
    items: [
      { href: "/", label: "Overview", icon: LayoutGrid },
      { href: "/live", label: "Live map", icon: MapIcon },
      { href: "/trips", label: "Trips", icon: Route },
    ],
  },
  {
    title: "Accounts",
    collapsible: true,
    icon: Users,
    items: [
      { href: "/drivers", label: "Drivers", icon: Car },
      { href: "/riders", label: "Riders", icon: Users },
    ],
  },
  {
    title: "Pricing",
    collapsible: true,
    icon: Tag,
    items: [
      { href: "/pricing", label: "Vehicle types", icon: Tag },
      { href: "/areas", label: "Regions", icon: Hexagon },
      { href: "/pricing/geofences", label: "Geofences", icon: MapPinned },
      { href: "/pricing/fees", label: "Fees and profiles", icon: Percent },
      { href: "/pricing/engines", label: "Engines", icon: Cpu },
    ],
  },
  {
    title: "Money",
    collapsible: true,
    icon: Receipt,
    items: [{ href: "/finance", label: "Finance", icon: Receipt }],
  },
  {
    title: "Trust",
    collapsible: true,
    icon: ShieldCheck,
    items: [
      { href: "/compliance/documents", label: "Documents", icon: FileCheck },
      { href: "/compliance/regulator", label: "Regulator", icon: Landmark },
      { href: "/compliance/fraud", label: "Fraud flags", icon: ScanSearch },
      { href: "/compliance/privacy", label: "Privacy", icon: LockKeyhole },
    ],
  },
  {
    title: "Tools",
    collapsible: true,
    icon: Wrench,
    items: [
      { href: "/safety", label: "Safety alerts", icon: ShieldAlert },
      { href: "/audit", label: "Audit log", icon: FileClock },
      { href: "/staff", label: "Staff", icon: UserCog },
      { href: "/staff/roles", label: "Roles", icon: KeyRound },
    ],
  },
  {
    title: "Settings",
    collapsible: true,
    icon: Settings,
    items: [{ href: "/settings/branding", label: "Branding", icon: Palette }],
  },
];

/** True when `href` is the current section (Overview only on "/"). */
export function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  if (href === "/pricing")
    return (
      pathname === "/pricing" ||
      (pathname.startsWith("/pricing/") && !/^\/pricing\/(fees|engines|geofences)/.test(pathname))
    );
  if (href === "/staff")
    return pathname === "/staff" || (pathname.startsWith("/staff/") && !pathname.startsWith("/staff/roles"));
  return pathname === href || pathname.startsWith(`${href}/`);
}
