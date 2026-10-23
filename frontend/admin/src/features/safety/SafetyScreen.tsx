"use client";
/** SafetyScreen — SOS alerts: who pressed it, on which trip, where, and closing it with what was done. */
import { MapPin, Phone, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Tabs } from "@/components/ui/Tabs";
import { useMe } from "@/features/me";
import { when } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { useResolveSos, useSosAlerts } from "./hooks";

export function SafetyScreen() {
  return (
    <Suspense>
      <Safety />
    </Suspense>
  );
}

function Safety() {
  const url = useUrlState();
  const view = (url.get("view") ?? "open") as "open" | "all";
  const { data, isLoading, error } = useSosAlerts(view === "all");
  const resolve = useResolveSos();
  const me = useMe();
  const [closing, setClosing] = useState<string>();
  return (
    <div>
      <PageHeader
        title="Safety alerts"
        subtitle="Riders who pressed SOS. Their trusted contacts were texted a live link. Call the rider, then close the alert with what you did."
      />
      <div className="mb-4">
        <Tabs
          id="sos"
          value={view}
          options={[
            { value: "open", label: "Open" },
            { value: "all", label: "All" },
          ]}
          onChange={(v) => url.set({ view: v === "open" ? undefined : v })}
        />
      </div>
      <ErrorNote error={error ?? resolve.error} />
      {isLoading ? (
        <Skeleton className="h-48" />
      ) : !data?.length ? (
        <Card index={1}>
          <Empty
            icon={<ShieldCheck size={20} />}
            title={view === "open" ? "No open alerts" : "No alerts yet"}
            body="When a rider presses SOS it appears here within seconds."
          />
        </Card>
      ) : (
        <div className="grid gap-3 xl:grid-cols-2">
          {data.map((a, i) => (
            <Card key={a.id} index={i + 1}>
              <div className="flex items-start gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    {a.resolved_at ? <Chip tone="ok">Closed</Chip> : <Chip tone="bad">Open</Chip>}
                    <span className="tnum text-sm text-muted">{when(a.created_at)}</span>
                  </div>
                  <p className="mt-2 text-lg font-semibold">{a.user_name ?? a.user_phone}</p>
                  <p className="text-sm text-muted">{`Trip ${a.trip_code} · ${a.contacts_alerted} trusted contact${a.contacts_alerted === 1 ? "" : "s"} texted`}</p>
                  {a.resolution ? (
                    <p className="mt-2 rounded-[10px] bg-surface-2 px-3 py-2 text-sm">{a.resolution}</p>
                  ) : null}
                </div>
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                <a
                  href={`tel:${a.user_phone}`}
                  className="inline-flex h-10 items-center gap-2 rounded-full bg-nav px-4 text-sm font-semibold text-nav-ink"
                >
                  <Phone size={15} aria-hidden="true" />
                  {`Call ${a.user_phone}`}
                </a>
                {a.lat && a.lng ? (
                  <a
                    href={`https://www.openstreetmap.org/?mlat=${a.lat}&mlon=${a.lng}#map=17/${a.lat}/${a.lng}`}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex h-10 items-center gap-2 rounded-full bg-surface-2 px-4 text-sm font-semibold"
                  >
                    <MapPin size={15} aria-hidden="true" />
                    Where they were
                  </a>
                ) : null}
                <Link
                  href={`/trips?trip=${a.trip_id}`}
                  className="inline-flex h-10 items-center rounded-full bg-surface-2 px-4 text-sm font-semibold"
                >
                  Open trip
                </Link>
                {!a.resolved_at && me.can("safety.resolve") ? (
                  <Button variant="brand" onClick={() => setClosing(a.id)}>
                    Close alert
                  </Button>
                ) : null}
              </div>
            </Card>
          ))}
        </div>
      )}
      <ReasonDialog
        open={!!closing}
        title="Close this alert?"
        body="Say what happened and what you did."
        confirm="Close alert"
        busy={resolve.isPending}
        onClose={() => setClosing(undefined)}
        onConfirm={(resolution) =>
          closing && resolve.mutate({ id: closing, resolution }, { onSuccess: () => setClosing(undefined) })
        }
      />
    </div>
  );
}
