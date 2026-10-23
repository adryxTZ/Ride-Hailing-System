"use client";
/**
 * TopBar — the bar above every page: the country and region the dashboard looks
 * at on the left; quick actions (refresh, safety alerts) and the signed-in
 * person's menu on the right. Sticky, on the canvas tone.
 */
import { useQueryClient } from "@tanstack/react-query";
import { Bell, RefreshCw } from "lucide-react";
import { useRouter } from "next/navigation";
import { useOpenSos } from "@/features/safety/hooks";
import { ScopePicker } from "./ScopePicker";
import { UserMenu } from "./UserMenu";

export function TopBar() {
  const router = useRouter();
  const qc = useQueryClient();
  const sos = useOpenSos();
  return (
    <header className="sticky top-0 z-30 flex flex-wrap items-center gap-3 bg-canvas/90 px-4 py-3 backdrop-blur sm:px-6">
      <ScopePicker />
      <div className="ml-auto flex items-center gap-2">
        <div className="flex items-center gap-1 rounded-[10px] bg-surface p-1">
          <IconButton label="Refresh data" onClick={() => void qc.invalidateQueries()}>
            <RefreshCw size={17} aria-hidden="true" />
          </IconButton>
          <IconButton
            label={sos ? `${sos} open safety alerts` : "Safety alerts"}
            onClick={() => router.push("/safety")}
          >
            <Bell size={17} aria-hidden="true" />
            {sos ? <span className="absolute right-1.5 top-1.5 size-2 rounded-full bg-critical" /> : null}
          </IconButton>
        </div>
        <UserMenu />
      </div>
    </header>
  );
}

function IconButton({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className="relative grid size-9 place-items-center rounded-[8px] text-ink transition-[background-color,transform] duration-150 hover:bg-surface-2 active:scale-[.96]"
    >
      {children}
    </button>
  );
}
