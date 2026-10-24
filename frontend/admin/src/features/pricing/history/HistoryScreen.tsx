"use client";
/** HistoryScreen — pricing history: the assignment timeline, "as of" a time, and the records export. */
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { Suspense } from "react";
import { OperationsHeader } from "@/components/ui/Operations";
import { Skeleton } from "@/components/ui/States";
import { Tabs } from "@/components/ui/Tabs";
import { useScope } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { AsOfView } from "./AsOfView";
import { ExportCard } from "./ExportCard";
import { TimelineView } from "./TimelineView";

const TABS = [
  { value: "timeline", label: "Timeline" },
  { value: "as-of", label: "As of a time" },
  { value: "export", label: "Export" },
] as const;
type Tab = (typeof TABS)[number]["value"];

export function HistoryScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <History />
    </Suspense>
  );
}

function History() {
  const { scope } = useScope();
  const url = useUrlState();
  const tab = (TABS.find((t) => t.value === url.get("tab"))?.value ?? "timeline") as Tab;
  return (
    <div>
      <OperationsHeader
        title="Pricing history"
        description="Which profile applied where and when, who changed it and why. Every trip also keeps its own pricing record."
        actions={
          <Link
            href="/pricing/fees"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
          >
            <ArrowLeft size={14} aria-hidden="true" /> Pricing profiles
          </Link>
        }
      />
      <div className="mb-4">
        <Tabs
          id="pricing-history"
          value={tab}
          options={[...TABS]}
          onChange={(v) => url.set({ tab: v === "timeline" ? undefined : v })}
        />
      </div>
      {tab === "timeline" ? <TimelineView country={scope.country} /> : null}
      {tab === "as-of" ? <AsOfView country={scope.country} /> : null}
      {tab === "export" ? <ExportCard country={scope.country} /> : null}
    </div>
  );
}
